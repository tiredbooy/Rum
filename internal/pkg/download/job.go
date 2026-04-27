package download

import (
	"context"
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
	Downloaded    int64              `json:"downloaded"`
	TotalSize     int64              `json:"total_size"`
	Speed         float64            `json:"speed"`
	RemainingTime time.Duration      `json:"remaining_time"`
	Error         error              `json:"error"`
	CancelFunc    context.CancelFunc `json:"-"`
}

func (j *Job) GetFileName() string     { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.FileName }
func (j *Job) SetFileName(name string) { j.Mu.Lock(); defer j.Mu.Unlock(); j.FileName = name }
func (j *Job) GetStatus() string       { j.Mu.RLock(); defer j.Mu.RUnlock(); return j.Status }
func (j *Job) SetStatus(s string)      { j.Mu.Lock(); defer j.Mu.Unlock(); j.Status = s }
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
