package download

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusPaused    = "paused"
	StatusCompleted = "completed"
	StatusError     = "error"
)

type Job struct {
	Mu            sync.RWMutex
	ID            string             `json:"id"`
	URL           string             `json:"url"`
	FileName      string             `json:"file_name"`
	OutputPath    string             `json:"output_path"`
	Status        string             `json:"status"`
	Priority      string             `json:"priority"`
	Downloaded    int64              `json:"downloaded"`
	TotalSize     int64              `json:"total_size"`
	ContentType   string             `json:"content_type"`
	SupportRange  bool               `json:"support_range"`
	Speed         float64            `json:"speed"`
	RemainingTime time.Duration      `json:"remaining_time"`
	Error         error              `json:"error"`
	BatchID       string             `json:"batch_id"`
	CancelFunc    context.CancelFunc `json:"-"`
	CreatedAt     time.Time          `json:"created_at"`
	CompletedAt   time.Time          `json:"completed_at"`
	// StartAt, when non-zero, defers automatic start of this job until the time
	// passes (honored by the schedule controller when scheduled-start is on). A
	// zero value means start immediately via the normal flow.
	StartAt time.Time `json:"start_at,omitempty"`

	// Per-job download options threaded from the create/batch request. These
	// overlay the manager's global Options when the job runs. Empty values mean
	// "use the global / default behavior".
	Checksum     string `json:"checksum,omitempty"`
	ChecksumAlgo string `json:"checksum_algo,omitempty"`
	Category     string `json:"category,omitempty"` // forces a category; "" = auto-detect

	// IntegrityHash is the hex digest of the fully assembled file, recorded on
	// completion when integrity verification was enabled. It lets a later verify
	// re-check the on-disk bytes without contacting the server, and replaces the
	// old practice of leaving a visible *.rumparts sidecar next to every finished
	// download (clutter). IntegrityHashAlgo names the algorithm ("sha256").
	IntegrityHash     string `json:"integrity_hash,omitempty"`
	IntegrityHashAlgo string `json:"integrity_hash_algo,omitempty"`
}

func (j *Job) GetOutputPath() string   { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.OutputPath }
func (j *Job) SetOutputPath(p string)  { j.Mu.Lock(); defer j.Mu.Unlock(); j.OutputPath = p }
func (j *Job) GetFileName() string     { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.FileName }
func (j *Job) SetFileName(name string) { j.Mu.Lock(); defer j.Mu.Unlock(); j.FileName = name }
func (j *Job) GetStatus() string       { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Status }
func (j *Job) SetStatus(s string)      { j.Mu.Lock(); defer j.Mu.Unlock(); j.Status = s }
func (j *Job) GetPriority() string     { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Priority }
func (j *Job) SetPriority(p string)    { j.Mu.Lock(); defer j.Mu.Unlock(); j.Priority = p }
func (j *Job) GetURL() string          { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.URL }
func (j *Job) GetDownloaded() int64    { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Downloaded }
func (j *Job) SetDownloaded(v int64)   { j.Mu.Lock(); defer j.Mu.Unlock(); j.Downloaded = v }
func (j *Job) GetTotalSize() int64     { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.TotalSize }
func (j *Job) SetTotalSize(v int64)    { j.Mu.Lock(); defer j.Mu.Unlock(); j.TotalSize = v }
func (j *Job) GetSpeed() float64       { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Speed }
func (j *Job) SetSpeed(v float64)      { j.Mu.Lock(); defer j.Mu.Unlock(); j.Speed = v }
func (j *Job) GetRemainingTime() time.Duration {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return j.RemainingTime
}
func (j *Job) SetRemainingTime(d time.Duration) {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	j.RemainingTime = d
}
func (j *Job) GetError() error  { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Error }
func (j *Job) SetError(e error) { j.Mu.Lock(); defer j.Mu.Unlock(); j.Error = e }

func (j *Job) GetCreatedAt() time.Time  { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.CreatedAt }
func (j *Job) SetCreatedAt(v time.Time) { j.Mu.Lock(); defer j.Mu.Unlock(); j.CreatedAt = v }

func (j *Job) GetCompletedAt() time.Time  { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.CompletedAt }
func (j *Job) SetCompletedAt(v time.Time) { j.Mu.Lock(); defer j.Mu.Unlock(); j.CompletedAt = v }

func (j *Job) GetStartAt() time.Time  { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.StartAt }
func (j *Job) SetStartAt(v time.Time) { j.Mu.Lock(); defer j.Mu.Unlock(); j.StartAt = v }

func (j *Job) GetChecksum() string      { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Checksum }
func (j *Job) SetChecksum(v string)     { j.Mu.Lock(); defer j.Mu.Unlock(); j.Checksum = v }
func (j *Job) GetChecksumAlgo() string  { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.ChecksumAlgo }
func (j *Job) SetChecksumAlgo(v string) { j.Mu.Lock(); defer j.Mu.Unlock(); j.ChecksumAlgo = v }
func (j *Job) GetCategory() string      { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Category }
func (j *Job) SetCategory(v string)     { j.Mu.Lock(); defer j.Mu.Unlock(); j.Category = v }

func (j *Job) GetIntegrityHash() string { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.IntegrityHash }

// SetIntegrityHash records the full-file digest and its algorithm together.
func (j *Job) SetIntegrityHash(hash, algo string) {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	j.IntegrityHash = hash
	j.IntegrityHashAlgo = algo
}

func (j *Job) GetCancelFunc() context.CancelFunc {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return j.CancelFunc
}
func (j *Job) SetCancelFunc(c context.CancelFunc) {
	j.Mu.Lock()
	defer j.Mu.Unlock()
	j.CancelFunc = c
}

// jobJSON mirrors Job's JSON-tagged fields (minus the mutex and the CancelFunc
// that is json:"-") so the Job can be marshaled while holding its read lock
// without copying the embedded sync.RWMutex (which `go vet` rightly forbids).
// The field tags match Job exactly, so the on-disk format is unchanged.
type jobJSON struct {
	ID                string        `json:"id"`
	URL               string        `json:"url"`
	FileName          string        `json:"file_name"`
	OutputPath        string        `json:"output_path"`
	Status            string        `json:"status"`
	Priority          string        `json:"priority"`
	Downloaded        int64         `json:"downloaded"`
	TotalSize         int64         `json:"total_size"`
	ContentType       string        `json:"content_type"`
	SupportRange      bool          `json:"support_range"`
	Speed             float64       `json:"speed"`
	RemainingTime     time.Duration `json:"remaining_time"`
	Error             error         `json:"error"`
	BatchID           string        `json:"batch_id"`
	CreatedAt         time.Time     `json:"created_at"`
	CompletedAt       time.Time     `json:"completed_at"`
	StartAt           time.Time     `json:"start_at,omitempty"`
	Checksum          string        `json:"checksum,omitempty"`
	ChecksumAlgo      string        `json:"checksum_algo,omitempty"`
	Category          string        `json:"category,omitempty"`
	IntegrityHash     string        `json:"integrity_hash,omitempty"`
	IntegrityHashAlgo string        `json:"integrity_hash_algo,omitempty"`
}

// MarshalJSON serializes a Job under its read lock. Without this, persisting the
// job set (saveToDisk) reads job fields via reflection while a running download
// concurrently mutates them (status, downloaded, output path), which the race
// detector flags. Taking Mu.RLock here pairs with the field setters' Mu.Lock to
// make persistence race-free.
func (j *Job) MarshalJSON() ([]byte, error) {
	j.Mu.RLock()
	defer j.Mu.RUnlock()
	return json.Marshal(jobJSON{
		ID:                j.ID,
		URL:               j.URL,
		FileName:          j.FileName,
		OutputPath:        j.OutputPath,
		Status:            j.Status,
		Priority:          j.Priority,
		Downloaded:        j.Downloaded,
		TotalSize:         j.TotalSize,
		ContentType:       j.ContentType,
		SupportRange:      j.SupportRange,
		Speed:             j.Speed,
		RemainingTime:     j.RemainingTime,
		Error:             j.Error,
		BatchID:           j.BatchID,
		CreatedAt:         j.CreatedAt,
		CompletedAt:       j.CompletedAt,
		StartAt:           j.StartAt,
		Checksum:          j.Checksum,
		ChecksumAlgo:      j.ChecksumAlgo,
		Category:          j.Category,
		IntegrityHash:     j.IntegrityHash,
		IntegrityHashAlgo: j.IntegrityHashAlgo,
	})
}
