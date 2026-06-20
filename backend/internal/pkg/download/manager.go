package download

import (
	"context"
	"fmt"
	"log"
	"mime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gen2brain/beeep"
	"github.com/google/uuid"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
	"github.com/tiredbooy/Rum/backend/internal/pkg/format"
	"github.com/tiredbooy/Rum/backend/internal/pkg/scheduler"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

// testCompletionHook, when non-nil, is invoked with a job id right after the job
// finishes (success, error, or pause). It exists only so tests can observe
// completion ordering without depending on disk persistence; production code
// leaves it nil. Access is guarded by testHookMu so a download goroutine reading
// the hook never races a test reassigning it.
var (
	testHookMu         sync.Mutex
	testCompletionHook func(id string)
)

func setTestCompletionHook(f func(id string)) func(id string) {
	testHookMu.Lock()
	defer testHookMu.Unlock()
	prev := testCompletionHook
	testCompletionHook = f
	return prev
}

func fireTestCompletionHook(id string) {
	testHookMu.Lock()
	f := testCompletionHook
	testHookMu.Unlock()
	if f != nil {
		f(id)
	}
}

type JobManager struct {
	mu             sync.RWMutex
	jobs           map[string]*Job
	urls           map[string]string
	opt            *Options
	subMu          sync.RWMutex
	subscribers    map[string][]chan dto.ProgressUpdate
	allSubscribers []chan dto.ProgressUpdate
	sortBy         SortField
	batchCounters  map[string]int32
	batchMu        sync.Mutex
	config         config.Setting

	// sched is the priority-aware concurrency gate. At most opt.Parallel jobs run
	// at once; queued jobs are granted highest-priority-first. A single dispatcher
	// goroutine pulls runnable ids from it and launches downloads.
	sched          *scheduler.Scheduler
	dispatchCtx    context.Context
	dispatchCancel context.CancelFunc
	dispatchWG     sync.WaitGroup
	shutdownOnce   sync.Once
}

type SortField string

var (
	statusPriority = map[string]int{
		StatusRunning:   1,
		StatusPending:   2,
		StatusPaused:    3,
		StatusCompleted: 4,
		StatusError:     5,
	}
	SortByName      SortField = "name"
	SortByCreatedAt SortField = "created_at"
)

type DashboardStats struct {
	ActiveDownloads   int     `json:"active_downloads"`
	CompletedToday    int     `json:"completed_today"`
	DownloadedTodayGB float64 `json:"downloaded_today_gb"`
	CurrentSpeedMBps  float64 `json:"current_speed_mbps"`
}

func NewJobManager(opt *Options) *JobManager {
	if opt == nil {
		opt = &Options{Parallel: 1, SpeedLimit: 0}
	}
	if opt.Parallel < 1 {
		opt.Parallel = 1
	}
	// Enable segmented (multi-connection) downloads by default when the caller
	// did not specify a connection count. Setting Connections to 1 explicitly
	// keeps the legacy single-stream behavior.
	if opt.Connections == 0 {
		opt.Connections = defaultConnections
	}

	var setting config.Setting
	setting.LoadSettingMetadata()

	dispatchCtx, dispatchCancel := context.WithCancel(context.Background())

	m := &JobManager{
		jobs:           make(map[string]*Job),
		urls:           make(map[string]string),
		opt:            opt,
		subscribers:    make(map[string][]chan dto.ProgressUpdate),
		batchCounters:  make(map[string]int32),
		config:         setting,
		sched:          scheduler.New(opt.Parallel),
		dispatchCtx:    dispatchCtx,
		dispatchCancel: dispatchCancel,
	}

	m.mu.Lock()
	m.loadFromDisk()
	m.mu.Unlock()

	// Start the single dispatcher goroutine. It is the only goroutine that ever
	// launches a download, so concurrency is bounded by the scheduler. It exits
	// when Shutdown cancels dispatchCtx / closes the scheduler (leak-free).
	m.dispatchWG.Add(1)
	go m.dispatch()

	return m
}

// dispatch is the single goroutine that turns scheduler grants into running
// downloads. It blocks in Next until a slot is free and a job is queued, then
// launches that job. Each launched download's defer calls sched.Done so the slot
// is always released. It returns when the scheduler is closed (Shutdown).
func (m *JobManager) dispatch() {
	defer m.dispatchWG.Done()
	for {
		id, ok := m.sched.Next(m.dispatchCtx)
		if !ok {
			return // scheduler closed or context cancelled
		}

		// Re-check the job is still eligible at the moment a slot opens. It may
		// have been deleted, completed, or paused while queued; if so, release the
		// slot and move on instead of (re)starting it.
		m.mu.Lock()
		job, exists := m.jobs[id]
		if !exists {
			m.mu.Unlock()
			m.sched.Done(id)
			continue
		}
		status := job.GetStatus()
		if status != StatusPending && status != StatusPaused {
			m.mu.Unlock()
			m.sched.Done(id)
			continue
		}
		// Establish the per-job cancellable context now (before running) so PauseJob
		// can cancel it once the job is running.
		bgCtx, cancel := context.WithCancel(context.Background())
		job.SetStatus(StatusRunning)
		job.SetCancelFunc(cancel)
		m.mu.Unlock()

		// release frees the scheduler slot. It is called as soon as the transfer
		// finishes (before slow post-completion side effects like notifications),
		// and again via defer as a panic-safe net. sync.Once makes it idempotent so
		// the slot is freed exactly once.
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { m.sched.Done(id) }) }

		go func(id string, ctx context.Context, cancel context.CancelFunc, release func()) {
			defer release()
			m.runDownload(ctx, id, cancel, release)
		}(id, bgCtx, cancel, release)
	}
}

func (m *JobManager) loadFromDisk() {
	LoadJobsFromDisk(m.jobs, m.urls)
}

func (m *JobManager) saveToDisk() {
	SaveJobsToDisk(m.jobs)
}

func (m *JobManager) GetJobIDByURL(url string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.urls[url]
	return id, ok
}

// JobCreateOptions carries optional per-job fields threaded from the create /
// batch API onto newly created jobs. All fields are optional; zero values mean
// "no override".
type JobCreateOptions struct {
	Checksum     string
	ChecksumAlgo string
	Category     string
	StartAt      time.Time
}

// CreateJobsFromURLsWithOptions creates jobs for the given URLs (same dedupe /
// HEAD / validation behavior as CreateJobsFromURLs) and stamps the optional
// per-job fields (checksum, category, scheduled start) onto each created job,
// persisting them. It returns the newly created jobs.
func (m *JobManager) CreateJobsFromURLsWithOptions(urls []string, opts JobCreateOptions) ([]*Job, error) {
	jobs, err := m.CreateJobsFromURLs(urls)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return jobs, nil
	}

	for _, job := range jobs {
		if opts.Checksum != "" {
			job.SetChecksum(opts.Checksum)
			job.SetChecksumAlgo(opts.ChecksumAlgo)
		}
		if opts.Category != "" {
			job.SetCategory(opts.Category)
		}
		if !opts.StartAt.IsZero() {
			job.SetStartAt(opts.StartAt)
		}
	}

	m.mu.Lock()
	m.saveToDisk()
	m.mu.Unlock()
	return jobs, nil
}

func (m *JobManager) CreateJobsFromURLs(urls []string) ([]*Job, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("no URLs provided")
	}

	// 1. Filter already known URLs
	m.mu.RLock()
	var toHead []string
	for _, u := range urls {
		// Reject unsupported schemes / malformed URLs at the edge (http/https
		// only). This blocks file://, data:, etc. before any network or disk work.
		if err := ValidateURL(u); err != nil {
			log.Printf("skipping invalid URL %q: %v", u, err)
			continue
		}
		if _, exists := m.urls[u]; !exists {
			toHead = append(toHead, u)
		}
	}
	m.mu.RUnlock()

	if len(toHead) == 0 {
		return nil, nil
	}

	// 2. Prepare downloader with current user agent
	userAgent := m.opt.UserAgent
	if userAgent == "" {
		userAgent = utils.GetRandomUserAgent()
	}
	downloader := NewDownloader(userAgent, m.opt.Referer)

	// 3. Run HEAD requests with bounded concurrency and a per-request timeout.
	//
	// The timeout is enforced via context.WithTimeout on the HTTP request
	// itself (HeadWithFallbackContext), so a slow/hung probe makes the
	// underlying Do() return promptly. The previous implementation raced an
	// inner goroutine against time.After, leaking the goroutine (and its TCP
	// connection) whenever the probe hung past the timeout.
	var maxConcurrent int = 5
	if m.opt.Parallel > 5 {
		maxConcurrent = m.opt.Parallel
	}
	if maxConcurrent < 1 {
		maxConcurrent = 5
	}
	const perRequestTimeout = 60 * time.Second

	type headRes struct {
		url  string
		info *HeaderInfo
		err  error
	}
	results := make([]headRes, len(toHead))

	g, gctx := errgroup.WithContext(context.Background())
	g.SetLimit(maxConcurrent)
	for i, u := range toHead {
		i, u := i, u
		g.Go(func() error {
			reqCtx, cancel := context.WithTimeout(gctx, perRequestTimeout)
			defer cancel()
			info, err := downloader.HeadWithFallbackContext(reqCtx, u)
			results[i] = headRes{url: u, info: info, err: err}
			return nil // probe failures are per-URL, never abort the group
		})
	}
	_ = g.Wait()

	// 4. Build jobs from successful HEAD responses
	var newJobs []*Job
	for _, res := range results {
		if res.err != nil {
			log.Printf("HEAD failed for %s: %v", res.url, res.err)
			continue
		}
		if res.info == nil {
			log.Printf("HEAD returned nil info for %s", res.url)
			continue
		}

		fileName := m.extractFileName(res.url, res.info)
		totalSize := utils.ConvertSizeToInt(res.info.ContentSize)
		if totalSize == 0 && !res.info.SupportsRange {
			// Might be a streaming URL – handle accordingly
			log.Printf("Warning: %s has zero size and no range support", res.url)
		}

		job := &Job{
			ID:           uuid.New().String(),
			URL:          res.url,
			OutputPath:   "",
			FileName:     fileName,
			TotalSize:    totalSize,
			ContentType:  res.info.ContentType,
			SupportRange: res.info.SupportsRange,
			Status:       StatusPending,
			CreatedAt:    time.Now(),
		}
		newJobs = append(newJobs, job)
	}

	if len(newJobs) == 0 {
		return nil, fmt.Errorf("no valid jobs could be created from %d URLs", len(toHead))
	}

	// 5. Add all new jobs atomically
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range newJobs {
		m.jobs[job.ID] = job
		m.urls[job.URL] = job.ID
	}
	m.saveToDisk()

	return newJobs, nil
}

func (m *JobManager) extractFileName(rawURL string, info *HeaderInfo) string {
	if info.ContentDisposition != "" {
		_, params, err := mime.ParseMediaType(info.ContentDisposition)
		if err == nil {
			if name := params["filename"]; name != "" {
				return name
			}
		}
	}
	if name := format.ExtractFileNameFromURL(rawURL); name != "" {
		return name
	}
	if name := format.CleanFileName(rawURL); name != "" && name != "/" {
		return name
	}
	return "downloaded.file"
}

func (m *JobManager) GetJob(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	return job, ok
}

func (m *JobManager) GetAllJobs() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}

	sort.Slice(jobs, func(i, j int) bool {
		if statusPriority[jobs[i].Status] != statusPriority[jobs[j].Status] {
			return statusPriority[jobs[i].Status] < statusPriority[jobs[j].Status]
		}

		return utils.TimeCompare(jobs[i].CreatedAt, jobs[j].CreatedAt)
	})

	return jobs
}

func (m *JobManager) CheckJobExists(ctx context.Context, jobID string) bool {
	m.mu.Lock()
	_, exists := m.jobs[jobID]
	if !exists {
		return false
	}
	m.mu.Unlock()
	return true
}

// StartJob enqueues a pending/paused job on the priority scheduler. The actual
// download is launched by the single dispatcher goroutine once a concurrency
// slot is free (highest priority first). It returns once the job is queued; the
// ctx argument is accepted for API compatibility but the dispatch is async.
func (m *JobManager) StartJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found", jobID)
	}
	if s := job.GetStatus(); s != StatusPending && s != StatusPaused {
		m.mu.Unlock()
		return fmt.Errorf("job %s is already %s", jobID, s)
	}
	// Mark pending so the dispatcher's eligibility re-check passes when the slot
	// opens. The dispatcher flips it to running just before launching.
	job.SetStatus(StatusPending)
	priority := job.GetPriority()
	m.mu.Unlock()

	m.sched.Submit(jobID, scheduler.ParsePriority(normalizePriority(priority)))
	return nil
}

// runDownload executes a single job. release frees the scheduler's concurrency
// slot; it is invoked as soon as the transfer finishes (before slow
// post-completion side effects such as desktop notifications) so a blocked
// notification cannot starve the gate. The job is already marked running by the
// dispatcher before this is called.
func (m *JobManager) runDownload(ctx context.Context, jobID string, cancel context.CancelFunc, release func()) {
	defer cancel()

	m.mu.RLock()
	job := m.jobs[jobID]
	m.mu.RUnlock()
	if job == nil {
		return
	}

	est := &speedEstimator{}
	var lastPublish time.Time
	const progressPublishInterval = 200 * time.Millisecond // ~5 frames/sec

	progressFn := func(downloaded, total int64) {
		now := time.Now()

		// Speed is sampled over a real time window (see speedEstimator). The engine
		// fires this callback on every buffer from every segment, so the old
		// per-callback delta — divided by the microseconds between two bursty
		// callbacks — produced phantom hundreds-of-MB/s readings on an otherwise
		// ~8 MB/s download. Keep the job's stored speed fresh on every call so the
		// polled stats endpoint sees it even between published frames.
		smoothSpeed := est.sample(downloaded, now)
		job.SetSpeed(smoothSpeed)

		// Throttle SSE progress frames to ~5/sec. This callback fires hundreds of
		// times/sec on a fast download; the frontend interpolates the bar between
		// frames, so a steady ~200ms cadence looks just as smooth at a fraction of
		// the CPU/IPC cost. The terminal 100% frame is always sent so the bar
		// reliably lands on complete.
		done := total > 0 && downloaded >= total
		if !done && !lastPublish.IsZero() && now.Sub(lastPublish) < progressPublishInterval {
			return
		}
		lastPublish = now

		var eta int64
		if smoothSpeed > 0 && total > 0 {
			remaining := total - downloaded
			if remaining > 0 {
				eta = int64(float64(remaining) / smoothSpeed)
			}
		}

		update := dto.ProgressUpdate{
			JobID:      jobID,
			Downloaded: downloaded,
			TotalSize:  total,
			Speed:      smoothSpeed,
			Status:     job.GetStatus(),
			Progress:   progressPercent(downloaded, total),
			ETA:        eta,
		}
		m.publishProgress(update)
	}

	// Build the effective per-download options: start from the manager's global
	// options (governor, parallelism, out dir, speed limit) and overlay the
	// per-job fields threaded from the create/batch request (checksum, category).
	m.mu.RLock()
	effectiveOpt := *m.opt
	m.mu.RUnlock()
	if cs := job.GetChecksum(); cs != "" {
		effectiveOpt.Checksum = cs
		effectiveOpt.ChecksumAlgo = job.GetChecksumAlgo()
	}
	if cat := job.GetCategory(); cat != "" {
		effectiveOpt.Categorize = true
		effectiveOpt.Category = cat
	} else if effectiveOpt.Categorize {
		// Global categorize-on (no explicit per-job category): keep auto-detect.
		effectiveOpt.Category = ""
	}

	err := DownloadSegmented(ctx, effectiveOpt, job, progressFn)

	m.mu.Lock()
	if ctx.Err() == context.Canceled {
		job.SetStatus(StatusPaused)
	} else if err == nil {
		job.SetStatus(StatusCompleted)
		job.SetCompletedAt(time.Now())
	} else {
		log.Println("ERROR STARTING: ", err.Error())
		job.SetStatus(StatusError)
		job.SetError(err)
		// KeepPartialOnFailure governs ONLY the failure path: when it is off, a
		// failed download's partial data + resume sidecar are discarded so a stale,
		// possibly-corrupt partial is not left around. When on (the default), the
		// partial is kept so the user can resume/repair. A pause/cancel above is not
		// a failure and always keeps the partial.
		if !effectiveOpt.KeepPartialOnFailure {
			RemovePartialData(job.GetOutputPath())
		}
	}
	downloaded := job.GetDownloaded()
	totalSize := job.GetTotalSize()
	finalStatus := job.GetStatus()
	m.mu.Unlock()

	finalUpdate := dto.ProgressUpdate{
		JobID:      jobID,
		Downloaded: downloaded,
		TotalSize:  totalSize,
		Speed:      0,
		Status:     finalStatus,
		Progress:   progressPercent(downloaded, totalSize),
		ETA:        0,
	}
	m.publishProgress(finalUpdate)

	// The transfer is done — free the scheduler slot now so the next queued job
	// can start immediately, before the (potentially slow / blocking) post-
	// completion side effects below (disk persist, OS notification, auto-open).
	if release != nil {
		release()
	}

	m.saveToDisk()

	m.onJobFinished(job)

	fireTestCompletionHook(jobID)
}

// ResumeInterruptedJobs re-queues jobs that were interrupted by a previous
// session's exit/crash so a restart picks up where it left off. It targets jobs
// persisted with status "running" (the app stopped mid-transfer) — these were
// never deliberately stopped by the user, so resuming them is safe and expected.
// Deliberately "paused" jobs are left alone (the user chose to pause them).
//
// It is the AutoResumeOnLaunch hook; the caller (server startup) gates it on the
// setting. The partial data on disk + the resume sidecar make this a real resume,
// not a restart-from-zero. Safe to call once on launch.
func (m *JobManager) ResumeInterruptedJobs(ctx context.Context) {
	m.mu.RLock()
	var toResume []string
	for id, job := range m.jobs {
		// A persisted "running" status means the previous session was killed before
		// it could flip the job to paused/completed/error — i.e. it was interrupted.
		if job.GetStatus() == StatusRunning {
			toResume = append(toResume, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range toResume {
		// Reset to pending so StartJob's eligibility check passes, then enqueue.
		m.mu.Lock()
		if job, ok := m.jobs[id]; ok {
			job.SetStatus(StatusPending)
		}
		m.mu.Unlock()
		_ = m.StartJob(ctx, id)
	}
}

// effectiveOptsForJob builds the per-download Options for a job the same way
// runDownload does: start from the manager's global Options (out dir, governor,
// connections, reliability settings) and overlay the job's per-job fields
// (checksum/algo, category). Used by verify/repair so they resolve the same
// output path + segment plan + validators a normal download would.
func (m *JobManager) effectiveOptsForJob(job *Job) Options {
	m.mu.RLock()
	opt := *m.opt
	m.mu.RUnlock()
	if cs := job.GetChecksum(); cs != "" {
		opt.Checksum = cs
		opt.ChecksumAlgo = job.GetChecksumAlgo()
	}
	if cat := job.GetCategory(); cat != "" {
		opt.Categorize = true
		opt.Category = cat
	}
	if opt.Downloader == nil {
		opt.Downloader = NewDownloader(opt.UserAgent, opt.Referer)
	}
	return opt
}

// VerifyJob runs an integrity check on a (typically completed) job and returns
// the result. It never mutates the file. deep forces a network re-validation
// (re-HEAD to confirm the source still matches). Returns ErrNotFound if the job
// is unknown so the handler can map it to 404.
func (m *JobManager) VerifyJob(ctx context.Context, id string, deep bool) (VerifyResult, error) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return VerifyResult{}, fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	opt := m.effectiveOptsForJob(job)
	return VerifyFile(ctx, opt, job, deep)
}

// RepairJob verifies a job and re-fetches only the bad/missing segments. When
// segments is nil it first runs VerifyFile to localize the damage, then repairs
// exactly those segments; an already-good job is a fast no-op re-verify. The job
// transitions running -> (repair) -> completed/error around the repair so the UI
// reflects progress. It is idempotent and safe on a completed job. Returns the
// post-repair VerifyResult. Returns ErrNotFound for an unknown job and ErrConflict
// if the job is actively downloading (pause it first).
func (m *JobManager) RepairJob(ctx context.Context, id string, segments []int) (VerifyResult, error) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	if !ok {
		return VerifyResult{}, fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	if job.GetStatus() == StatusRunning {
		return VerifyResult{}, fmt.Errorf("job %s is downloading; pause it before repair: %w", id, ErrConflict)
	}

	opt := m.effectiveOptsForJob(job)

	// Localize damage when the caller did not specify segments.
	if segments == nil {
		res, err := VerifyFile(ctx, opt, job, false)
		if err != nil {
			return VerifyResult{}, err
		}
		if res.Ok {
			// Nothing to repair; report the clean verdict (idempotent no-op).
			return res, nil
		}
		segments = res.BadSegments
	}

	prevStatus := job.GetStatus()
	job.SetStatus(StatusRunning)
	job.SetError(nil)

	if err := RepairFile(ctx, opt, job, segments); err != nil {
		// Restore a terminal status on failure so the card doesn't get stuck running.
		job.SetStatus(StatusError)
		job.SetError(err)
		m.saveToDisk()
		return VerifyResult{}, err
	}

	// Re-verify after the repair to confirm the file is now whole.
	res, err := VerifyFile(ctx, opt, job, false)
	if err != nil {
		job.SetStatus(StatusError)
		job.SetError(err)
		m.saveToDisk()
		return VerifyResult{}, err
	}
	if res.Ok {
		job.SetStatus(StatusCompleted)
		job.SetCompletedAt(time.Now())
	} else {
		// Still bad after a repair pass: surface it rather than claim success.
		job.SetStatus(prevStatus)
	}
	m.saveToDisk()
	return res, nil
}

func (m *JobManager) StartAllJobs(ctx context.Context) {
	batchID := fmt.Sprintf("batch_%d", time.Now().UnixNano())
	var eligibleJobs []*Job
	m.mu.RLock()

	for _, job := range m.jobs {
		if s := job.GetStatus(); s == StatusPending || s == StatusPaused {
			eligibleJobs = append(eligibleJobs, job)
		}
	}
	m.mu.RUnlock()

	m.batchMu.Lock()
	m.batchCounters[batchID] = int32(len(eligibleJobs))
	m.batchMu.Unlock()

	// Submit higher-priority jobs first; fall back to the configured sort within
	// the same priority level. The scheduler also orders by priority, but a
	// deterministic Submit order keeps FIFO-within-a-level behavior predictable.
	sort.SliceStable(eligibleJobs, func(i, j int) bool {
		pi, pj := priorityRank(eligibleJobs[i].GetPriority()), priorityRank(eligibleJobs[j].GetPriority())
		if pi != pj {
			return pi > pj
		}
		if m.sortBy == "created_at" {
			return utils.TimeCompare(eligibleJobs[i].GetCreatedAt(), eligibleJobs[j].GetCreatedAt())
		}

		return eligibleJobs[i].GetFileName() < eligibleJobs[j].GetFileName()
	})

	for _, job := range eligibleJobs {
		job.BatchID = batchID
		m.StartJob(ctx, job.ID)
	}
}

// SetSpeedLimit updates the global download speed limit (KB/s, 0 = unlimited)
// stored on the manager's Options. When a SpeedGovernor is attached (the desktop
// server path), the schedule controller calls Governor.SetLimitKBps so running
// downloads pick up the new limit live; new downloads always read this value.
func (m *JobManager) SetSpeedLimit(kbps int) {
	if kbps < 0 {
		kbps = 0
	}
	m.mu.Lock()
	m.opt.SpeedLimit = kbps
	m.mu.Unlock()
}

// SetMax adjusts the maximum number of concurrent downloads at runtime. It is a
// passthrough to the scheduler; n is clamped to >= 1 by the scheduler.
func (m *JobManager) SetMax(n int) {
	if n < 1 {
		n = 1
	}
	m.mu.Lock()
	m.opt.Parallel = n
	m.mu.Unlock()
	m.sched.SetMax(n)
}

// SetConnections updates the per-download parallel connection (segment) count
// used for NEW downloads; already-running downloads keep their existing segment
// plan (changing it mid-flight would invalidate the resume sidecar). n is clamped
// to [1, maxConnections]; 1 means single-stream.
func (m *JobManager) SetConnections(n int) {
	if n < 1 {
		n = 1
	}
	if n > maxConnections {
		n = maxConnections
	}
	m.mu.Lock()
	m.opt.Connections = n
	m.mu.Unlock()
}

// SetReliabilityOptions updates the reliability/UX engine options (integrity
// verification, retry backoff base, temp dir, keep-partial-on-failure) on the
// live manager so a settings change applies to the NEXT download without an app
// restart. Already-running downloads keep the options they started with.
func (m *JobManager) SetReliabilityOptions(verifyIntegrity bool, retryBackoffSec int, tempDir string, keepPartialOnFailure bool) {
	m.mu.Lock()
	m.opt.VerifyIntegrity = verifyIntegrity
	m.opt.RetryBackoffSec = retryBackoffSec
	m.opt.TempDir = tempDir
	m.opt.KeepPartialOnFailure = keepPartialOnFailure
	m.mu.Unlock()
}

// Shutdown stops the dispatcher and closes the scheduler. It cancels the
// dispatch context (waking a blocked Next), closes the scheduler (so no further
// Submit/Next succeed), and waits for the dispatcher goroutine to exit. In-flight
// downloads continue under their own contexts; callers that want to stop them
// should PauseAllJobs first. Shutdown is idempotent and safe to call once on app
// exit.
func (m *JobManager) Shutdown() {
	m.shutdownOnce.Do(func() {
		m.dispatchCancel()
		m.sched.Close()
		m.dispatchWG.Wait()
	})
}

func (m *JobManager) PauseJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}
	if job.GetStatus() != StatusRunning {
		return fmt.Errorf("job %s is not running", jobID)
	}
	if cancel := job.GetCancelFunc(); cancel != nil {
		cancel()
	}

	return nil
}

// normalizePriority maps any input to one of the three valid levels, defaulting
// to "normal". Validation of client input happens at the API edge (dto layer);
// this is the engine-side safety net.
func normalizePriority(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "low":
		return "low"
	case "high":
		return "high"
	default:
		return "normal"
	}
}

// priorityRank gives a sortable weight (higher = scheduled sooner).
func priorityRank(p string) int {
	switch normalizePriority(p) {
	case "high":
		return 2
	case "low":
		return 0
	default:
		return 1
	}
}

// SetJobPriority updates a job's scheduling priority. The new priority takes
// effect the next time the job is (re)started via StartAllJobs.
func (m *JobManager) SetJobPriority(id, priority string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	job.SetPriority(normalizePriority(priority))
	m.mu.Unlock()
	m.saveToDisk()
	return nil
}

// RetryJob restarts a failed/paused job. Any partial progress on disk is
// preserved, so the retry resumes rather than re-downloading from zero. A
// running job is a conflict (pause it first).
func (m *JobManager) RetryJob(ctx context.Context, id string) error {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found: %w", id, ErrNotFound)
	}
	if job.GetStatus() == StatusRunning {
		m.mu.Unlock()
		return fmt.Errorf("job %s is already running: %w", id, ErrConflict)
	}
	job.SetError(nil)
	job.SetStatus(StatusPending)
	m.mu.Unlock()

	return m.StartJob(ctx, id)
}

func (m *JobManager) PauseAllJobs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, job := range m.jobs {
		if cancel := job.GetCancelFunc(); cancel != nil {
			cancel()
		}
	}
	return nil
}

func (m *JobManager) Subscribe(jobID string) <-chan dto.ProgressUpdate {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	ch := make(chan dto.ProgressUpdate, 10)
	m.subscribers[jobID] = append(m.subscribers[jobID], ch)
	return ch
}

func (m *JobManager) UnSubscribe(jobID string, ch <-chan dto.ProgressUpdate) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	subscribers := m.subscribers[jobID]
	for i, sub := range subscribers {
		if sub == ch {
			m.subscribers[jobID] = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}

	if len(m.subscribers[jobID]) == 0 {
		delete(m.subscribers, jobID)
	}
}

func (m *JobManager) SubscribeAll() <-chan dto.ProgressUpdate {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	ch := make(chan dto.ProgressUpdate, 20)
	m.allSubscribers = append(m.allSubscribers, ch)
	return ch
}

func (m *JobManager) UnSubscribeAll(ch <-chan dto.ProgressUpdate) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for i, sub := range m.allSubscribers {
		if sub == ch {
			m.allSubscribers = append(m.allSubscribers[:i], m.allSubscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

func (m *JobManager) publishProgress(update dto.ProgressUpdate) {
	m.subMu.RLock()
	defer m.subMu.RUnlock()
	for _, ch := range m.subscribers[update.JobID] {
		select {
		case ch <- update:
		default:
		}
	}

	// All Downloads
	for _, ch := range m.allSubscribers {
		select {
		case ch <- update:
		default:

		}
	}
}

func (m *JobManager) DeleteJob(jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job id is not valid")
	}

	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found", jobID)
	}

	// Cancel an in-flight transfer first so we don't race the download goroutine
	// while removing its partial file (it would otherwise keep writing).
	if cancel := job.GetCancelFunc(); cancel != nil {
		cancel()
	}

	// An explicit delete always removes the on-disk partial + resume sidecar,
	// regardless of KeepPartialOnFailure (that flag only governs *failures*). A
	// completed download's final file is left in place — we only drop the resume
	// sidecar in that case (RemovePartialData removes the file too only if it is
	// still the partial; a finished file at the output path is the user's data, so
	// guard on status).
	outputPath := job.GetOutputPath()
	status := job.GetStatus()

	// Drop from the in-memory maps + URL index so a re-add isn't blocked by a stale
	// dedup entry.
	delete(m.jobs, jobID)
	delete(m.urls, job.URL)
	m.mu.Unlock()

	// Remove partial data outside the lock (disk I/O). A completed job keeps its
	// finished file; only its (usually already-removed) sidecar is cleaned.
	if status == StatusCompleted {
		removePartsMeta(outputPath)
	} else {
		RemovePartialData(outputPath)
	}

	if err := DeleteJobFromDisk(jobID); err != nil {
		return fmt.Errorf("failed to delete job from disk: %w", err)
	}

	return nil
}

// shouldDeleteForFilter reports whether a job matches a delete filter.
//
//	"all"               -> every job
//	"completed"         -> StatusCompleted
//	"error" / "failed"  -> StatusError
//
// An unknown filter matches nothing (safer than nuking everything on a typo).
func shouldDeleteForFilter(job *Job, filter string) bool {
	switch filter {
	case "all":
		return true
	case "completed":
		return job.GetStatus() == StatusCompleted
	case "error", "failed":
		return job.GetStatus() == StatusError
	default:
		return false
	}
}

// DeleteJobsByFilter deletes all jobs matching the status filter ("all",
// "completed", "error"/"failed") atomically w.r.t. the scheduler:
//   - A targeted job that is still running is cancelled FIRST so deletion is safe
//     (the download goroutine stops writing before we remove its partial).
//   - Each deleted job's on-disk partial + resume sidecar are removed (an explicit
//     delete always removes partials, regardless of KeepPartialOnFailure, which
//     only governs failures). A completed job's finished file is kept; only its
//     resume sidecar is cleaned.
//   - The surviving set + the URL dedup index are rebuilt under the lock so a
//     concurrent dispatch sees a consistent map.
func (m *JobManager) DeleteJobsByFilter(filter string) error {
	if filter == "" {
		filter = "all"
	}

	m.mu.Lock()

	// Partition into deleted vs. remaining, cancelling running jobs that are being
	// deleted so they stop writing before we remove their partials.
	type partialTarget struct {
		path      string
		completed bool
	}
	var toClean []partialTarget
	var remaining []*Job
	for _, job := range m.jobs {
		if shouldDeleteForFilter(job, filter) {
			if cancel := job.GetCancelFunc(); cancel != nil {
				cancel() // pause/cancel an in-flight transfer before deleting it
			}
			toClean = append(toClean, partialTarget{
				path:      job.GetOutputPath(),
				completed: job.GetStatus() == StatusCompleted,
			})
			continue
		}
		remaining = append(remaining, job)
	}

	if err := filesystem.WriteMetadataFile("jobs.json", remaining); err != nil {
		m.mu.Unlock()
		return err
	}

	m.jobs = make(map[string]*Job, len(remaining))
	// Rebuild the URL dedup index from the surviving jobs. Previously this was
	// reset to an empty map and never repopulated, corrupting URL dedup so
	// re-adding a still-present URL would create a duplicate job.
	m.urls = make(map[string]string, len(remaining))
	for _, job := range remaining {
		m.jobs[job.ID] = job
		m.urls[job.URL] = job.ID
	}
	m.mu.Unlock()

	// Remove partials outside the lock (disk I/O). A completed job keeps its
	// finished file; only its sidecar is cleaned.
	for _, t := range toClean {
		if t.completed {
			removePartsMeta(t.path)
		} else {
			RemovePartialData(t.path)
		}
	}

	return nil
}

func (m *JobManager) onJobFinished(job *Job) {
	if job.BatchID != "" {
		m.batchMu.Lock()
		remaining := m.batchCounters[job.BatchID] - 1
		if remaining == 0 {
			delete(m.batchCounters, job.BatchID)
			m.batchMu.Unlock()
			m.handleBatchCompletion(job.BatchID, job)
		} else {
			m.batchCounters[job.BatchID] = remaining
			m.batchMu.Unlock()
		}
	} else {
		m.completionOperations(job)
	}

}

func (m *JobManager) handleBatchCompletion(batchID string, job *Job) {
	m.completionOperations(job)
	m.batchMu.Lock()
	delete(m.batchCounters, batchID)
	m.batchMu.Unlock()
}

func (m *JobManager) completionOperations(job *Job) {
	// Use a local snapshot of the settings: multiple downloads can finish
	// concurrently, so mutating shared manager state (m.config) here would race.
	var setting config.Setting
	setting.LoadSettingMetadata()

	if setting.PostDownload.AutoOpenDir {
		filesystem.OpenFolder(job.GetOutputPath())
	}

	switch setting.PostDownload.Action {
	case "shutdown":
		if err := utils.ShutdownPC(); err != nil {
			log.Printf("Shutdown failed: %v", err)
		}
	case "sleep":
		if err := utils.SleepPC(); err != nil {
			log.Printf("Sleep failed: %v", err)
		}
	case "close":
		if quitFunc != nil {
			quitFunc()
		}
	}

	if !setting.Silent && job.GetStatus() == StatusCompleted {
		beeep.Beep(beeep.DefaultFreq, beeep.DefaultDuration)
		beeep.Notify("Downlods Completed", "All Jobs Finished", "")
	}
}

func (m *JobManager) GetDashboardStats() DashboardStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var active int
	var completedToday int
	var todayBytes int64
	var totalSpeed float64

	for _, job := range m.jobs {
		status := job.GetStatus()
		completedAt := job.GetCompletedAt()
		totalSize := job.GetTotalSize()
		speed := job.GetSpeed()

		if status == StatusRunning {
			active++
			totalSpeed += speed
		}

		if status == StatusCompleted && completedAt.After(todayStart) {
			completedToday++
			todayBytes += totalSize
		}
	}

	return DashboardStats{
		ActiveDownloads:   active,
		CompletedToday:    completedToday,
		DownloadedTodayGB: float64(todayBytes) / (1024 * 1024 * 1024),
		CurrentSpeedMBps:  totalSpeed / (1024 * 1024), // convert bytes/sec to MB/s
	}
}

// ActiveCount returns the number of currently-running jobs (used by the idle
// memory controller).
func (m *JobManager) ActiveCount() int {
	n := 0
	for _, j := range m.GetAllJobs() {
		if j.GetStatus() == StatusRunning {
			n++
		}
	}
	return n
}
