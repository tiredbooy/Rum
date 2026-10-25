package download

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// fakeDisk lets a test play the part of a volume: how much is free, and whether
// writes currently fail with ENOSPC after a byte budget.
type fakeDisk struct {
	free     atomic.Int64
	budget   atomic.Int64 // bytes writable before ENOSPC; <0 = unlimited
	written  atomic.Int64
	failures atomic.Int64
}

func installFakeDisk(t *testing.T, free, budget int64) *fakeDisk {
	t.Helper()
	d := &fakeDisk{}
	d.free.Store(free)
	d.budget.Store(budget)
	prevFree, prevFault := diskFreeFn, testWriteFault
	diskFreeFn = func(string) (int64, bool) { return d.free.Load(), true }
	testWriteFault = func(n int) error {
		b := d.budget.Load()
		if b < 0 {
			return nil
		}
		if d.written.Add(int64(n)) > b {
			d.failures.Add(1)
			// Same shape the os package returns for a failed write.
			return &os.PathError{Op: "write", Path: "x", Err: syscall.ENOSPC}
		}
		return nil
	}
	t.Cleanup(func() { diskFreeFn, testWriteFault = prevFree, prevFault })
	return d
}

func serveBytes(t *testing.T, size int) *httptest.Server {
	t.Helper()
	body := bytes.Repeat([]byte("abcdefgh"), size/8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, "f.bin", time.Time{}, bytes.NewReader(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func waitStatus(t *testing.T, j *Job, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if j.GetStatus() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status = %q (err %v), want %q", j.GetStatus(), j.GetError(), want)
}

func addDiskJob(m *JobManager, url string, size int64) *Job {
	j := &Job{ID: "j", URL: url + "/f.bin", FileName: "f.bin", TotalSize: size, SupportRange: true, Status: StatusPending}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.mu.Unlock()
	return j
}

func runDiskFullScenario(t *testing.T, connections int, size int64) {
	srv := serveBytes(t, int(size))
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.opt.Connections = connections
	m.opt.KeepPartialOnFailure = false // disk-full must keep the partial anyway
	disk := installFakeDisk(t, 100<<20, size/4)

	j := addDiskJob(m, srv.URL, size)
	if err := m.StartJob(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, j, StatusError)

	var df *DiskFullError
	if !errors.As(j.GetError(), &df) {
		t.Fatalf("error = %v, want *DiskFullError", j.GetError())
	}
	if ErrorCodeOf(j.GetError()) != ErrorCodeDiskFull {
		t.Fatalf("code = %q", ErrorCodeOf(j.GetError()))
	}
	if df.FreeBytes != 100<<20 {
		t.Errorf("free = %d, want %d", df.FreeBytes, 100<<20)
	}
	if df.NeededBytes <= 0 || df.NeededBytes > size {
		t.Errorf("needed = %d, want within (0,%d]", df.NeededBytes, size)
	}
	if df.Path == "" {
		t.Error("disk path empty")
	}
	// The partial (and the segmented sidecar) must survive.
	st, err := os.Stat(j.GetOutputPath())
	if err != nil {
		t.Fatalf("partial removed on disk-full: %v", err)
	}
	if connections > 1 {
		if st.Size() != size {
			t.Errorf("presized file lost: %d", st.Size())
		}
		if _, err := os.Stat(partsFilePath(j.GetOutputPath())); err != nil {
			t.Errorf("resume sidecar removed on disk-full: %v", err)
		}
	}

	// Space frees up: the watcher resumes it without user action.
	disk.budget.Store(-1)
	disk.free.Store(1 << 30)
	w := NewDiskWatcher(m)
	w.tick(context.Background())
	waitStatus(t, j, StatusCompleted)
	if got, _ := os.Stat(j.GetOutputPath()); got == nil || got.Size() != size {
		t.Fatalf("final file wrong: %v", got)
	}
}

func TestDiskFullMidDownloadSingleStream(t *testing.T) { runDiskFullScenario(t, 1, 1<<20) }
func TestDiskFullMidDownloadSegmented(t *testing.T)    { runDiskFullScenario(t, 4, 4<<20) }

func TestPreflightReturnsDiskFullErrorWithNumbers(t *testing.T) {
	installFakeDisk(t, 1<<20, -1)
	err := PreflightDiskSpace(t.TempDir(), 500<<20)
	var df *DiskFullError
	if !errors.As(err, &df) {
		t.Fatalf("got %v", err)
	}
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Error("must still wrap ErrInsufficientDiskSpace")
	}
	if df.NeededBytes != 500<<20 || df.FreeBytes != 1<<20 {
		t.Errorf("needed/free = %d/%d", df.NeededBytes, df.FreeBytes)
	}
}

// On resume the preflight asks for what is LEFT, not the whole file.
func TestResumePreflightUsesRemainingBytes(t *testing.T) {
	srv := serveBytes(t, 1<<20)
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	m.opt.Connections = 1
	installFakeDisk(t, 1<<10, -1)

	j := addDiskJob(m, srv.URL, 1<<20)
	if err := m.StartJob(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, j, StatusError)
	if err := os.WriteFile(j.GetOutputPath(), make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.RetryJob(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	// Wait for the second failure to land with the new numbers.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var df *DiskFullError
		if errors.As(j.GetError(), &df) && df.NeededBytes == (1<<20)-1000 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("needed never became remaining: %v", j.GetError())
}

func TestDiskFullSurvivesRestartAsError(t *testing.T) {
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	orig := &DiskFullError{NeededBytes: 5 << 30, FreeBytes: 2 << 30, Path: "/data/dl", Err: syscall.ENOSPC}
	reload(t, m, []*Job{{ID: "d", URL: "http://x/d", Status: StatusError, Error: orig}})

	j, _ := m.GetJob("d")
	if j.GetStatus() != StatusError || j.GetInterrupted() {
		t.Fatalf("status=%q interrupted=%v", j.GetStatus(), j.GetInterrupted())
	}
	var df *DiskFullError
	if !errors.As(j.GetError(), &df) || df.NeededBytes != 5<<30 || df.FreeBytes != 2<<30 || df.Path != "/data/dl" {
		t.Fatalf("lost after reload: %#v", j.GetError())
	}
	data, _ := j.MarshalJSON()
	var m2 map[string]any
	_ = json.Unmarshal(data, &m2)
	if m2["error_code"] != "disk_full" {
		t.Errorf("error_code not persisted: %s", data)
	}
}

func TestDiskFullIsNeverRetriedAutomatically(t *testing.T) {
	for _, err := range []error{
		&DiskFullError{NeededBytes: 1, Err: syscall.ENOSPC},
		&os.PathError{Op: "write", Err: syscall.ENOSPC},
		syscall.EDQUOT,
	} {
		if isRetryableError(err) {
			t.Errorf("%v must not be retryable (reconnect loop)", err)
		}
	}
}

func TestDiskWatcherWaitsForRealGrowth(t *testing.T) {
	m := newQuietManager(t)
	df := &DiskFullError{NeededBytes: 1 << 30, FreeBytes: 100 << 20, Path: t.TempDir()}
	job := addFailedJob(m, "w", "http://127.0.0.1:1/x", df)
	disk := installFakeDisk(t, 100<<20, -1)
	w := NewDiskWatcher(m)

	w.tick(context.Background()) // nothing changed
	if job.GetStatus() != StatusError {
		t.Fatalf("resumed without more space: %q", job.GetStatus())
	}
	disk.free.Store(1<<30 + 1<<20) // grew but still short of needed+margin
	w.tick(context.Background())
	if job.GetStatus() != StatusError {
		t.Fatalf("resumed while still short: %q", job.GetStatus())
	}
	disk.free.Store(2 << 30)
	w.tick(context.Background())
	if job.GetStatus() == StatusError {
		t.Fatal("not resumed after space grew")
	}
}

func TestCreateJobWithoutSpaceIsDiskFullImmediately(t *testing.T) {
	srv := serveBytes(t, 1<<20)
	m, _ := newManagerForTest(t, 1)
	defer m.Shutdown()
	installFakeDisk(t, 1<<10, -1)

	jobs, err := m.CreateJobsFromURLs([]string{srv.URL + "/new.bin"})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("create: %v %v", jobs, err)
	}
	var df *DiskFullError
	if jobs[0].GetStatus() != StatusError || !errors.As(jobs[0].GetError(), &df) {
		t.Fatalf("status=%q err=%v", jobs[0].GetStatus(), jobs[0].GetError())
	}
	if df.NeededBytes != 1<<20 {
		t.Errorf("needed = %d", df.NeededBytes)
	}
}
