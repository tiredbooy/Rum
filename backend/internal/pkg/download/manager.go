package download

import (
	"context"
	"fmt"
	"log"
	"mime"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
	"github.com/tiredbooy/Rum/backend/internal/pkg/format"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

var statusPriority = map[string]int{
	StatusRunning:   1,
	StatusPending:   2,
	StatusPaused:    3,
	StatusCompleted: 4,
	StatusError:     5,
}

type JobManager struct {
	mu             sync.RWMutex
	jobs           map[string]*Job
	urls           map[string]string
	sem            chan struct{}
	opt            *Options
	subMu          sync.RWMutex
	subscribers    map[string][]chan dto.ProgressUpdate
	allSubscribers []chan dto.ProgressUpdate
}

func NewJobManager(opt *Options) *JobManager {
	if opt == nil {
		opt = &Options{Parallel: 1, SpeedLimit: 0}
	}
	if opt.Parallel < 1 {
		opt.Parallel = 1
	}

	m := &JobManager{
		jobs:        make(map[string]*Job),
		urls:        make(map[string]string),
		sem:         make(chan struct{}, opt.Parallel),
		opt:         opt,
		subscribers: make(map[string][]chan dto.ProgressUpdate),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadFromDisk()
	return m
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

func (m *JobManager) CreateJobsFromURLs(urls []string) ([]*Job, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("No URLs Provided")
	}

	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	// 1. Filter out already known URLs
	m.mu.RLock()
	var toHead []string
	for _, u := range urls {
		if _, exists := m.urls[u]; !exists {
			toHead = append(toHead, u)
		}
	}
	m.mu.RUnlock()

	if len(toHead) == 0 {
		return nil, nil
	}

	// 2. Prepare downloader with current user agent (use random if empty)
	userAgent := m.opt.UserAgent
	if userAgent == "" {
		userAgent = utils.GetRandomUserAgent()
	}

	downloader := NewDownloader(userAgent, m.opt.Referer)

	// 3. Run HEAD requests concurrently
	maxConcurrent := m.opt.Parallel
	if maxConcurrent < 1 {
		maxConcurrent = 5
	}

	sem := make(chan struct{}, maxConcurrent)
	type headRes struct {
		url  string
		info *HeaderInfo
		err  error
	}
	resCh := make(chan headRes, len(toHead))
	for _, url := range toHead {
		sem <- struct{}{}
		go func(u string) {
			defer func() { <-sem }()
			info, err := downloader.HeadWithFallback(u)
			resCh <- headRes{url: u, info: info, err: err}
		}(url)
	}

	// 4. Build jobs from successfull HEAD resposnes
	var newJobs []*Job
	for i := 0; i < len(toHead); i++ {
		res := <-resCh
		if res.err != nil {
			log.Printf("Head failed for %s: %v", res.url, res.err)
			continue
		}

		fileName := m.extractFileName(res.url, res.info)
		job := &Job{
			ID:           uuid.New().String(),
			URL:          res.url,
			OutputPath:   m.opt.Out,
			FileName:     fileName,
			TotalSize:    utils.ConvertSizeToInt(res.info.ContentSize),
			ContentType:  res.info.ContentType,
			SupportRange: res.info.SupportsRange,
			Status:       StatusPending,
			CreatedAt:    time.Now(),
		}
		newJobs = append(newJobs, job)
	}

	// 5. Add new jobs to manager
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

func (m *JobManager) StartJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("job %s not found", jobID)
	}
	if job.Status != StatusPending && job.Status != StatusPaused {
		m.mu.Unlock()
		return fmt.Errorf("job %s is already %s", jobID, job.Status)
	}
	job.SetStatus(StatusPending)
	m.mu.Unlock()

	bgCtx, cancel := context.WithCancel(context.Background())
	job.CancelFunc = cancel

	select {
	case m.sem <- struct{}{}:
		log.Printf("StartJob: acquired semaphore for %s", jobID)
	case <-ctx.Done():
		return ctx.Err()
	}

	go func() {
		defer func() { <-m.sem }()
		defer cancel()

		log.Printf("StartJob: goroutine running for %s", jobID)

		m.mu.Lock()
		job.SetStatus(StatusRunning)
		m.mu.Unlock()

		var (
			lastDownloaded int64
			lastTime       time.Time
		)

		progressFn := func(downloaded, total int64) {
			now := time.Now()

			var speed int64
			if !lastTime.IsZero() {
				elapsed := now.Sub(lastTime).Seconds()
				if elapsed > 0 {
					speed = int64(float64(downloaded-lastDownloaded) / elapsed)
				}
			}

			lastDownloaded = downloaded
			lastTime = now

			update := dto.ProgressUpdate{
				JobID:      jobID,
				Downloaded: downloaded,
				TotalSize:  total,
				Speed:      float64(speed),
				Status:     string(job.Status),
			}
			if total > 0 {
				update.Progress = int(float64(downloaded) / float64(total) * 100)
			}
			m.publishProgress(update)
		}

		err := DownloadSingleFile(bgCtx, *m.opt, job, progressFn)

		finalUpdate := dto.ProgressUpdate{
			JobID:      jobID,
			Downloaded: job.GetDownloaded(),
			TotalSize:  job.TotalSize,
			Speed:      0,
			Status:     string(job.Status),
		}
		if job.TotalSize > 0 {
			finalUpdate.Progress = int(float64(job.GetDownloaded()) / float64(job.GetTotalSize()) * 100)
		}
		m.publishProgress(finalUpdate)

		m.mu.Lock()
		defer m.mu.Unlock()
		if bgCtx.Err() == context.Canceled {
			job.SetStatus(StatusPaused)
			log.Printf("Job %s paused", jobID)
		} else if err == nil {
			job.SetStatus(StatusCompleted)
			log.Printf("Job %s completed", jobID)
		} else {
			job.SetStatus(StatusError)
			job.Error = err
			log.Printf("Job %s error: %v", jobID, err)
		}
		m.saveToDisk()
	}()
	return nil
}

func (m *JobManager) StartAllJobs(ctx context.Context) {
	m.mu.RLock()
	var jobIDs []string
	for id, job := range m.jobs {
		if job.Status == StatusPending || job.Status == StatusPaused {
			jobIDs = append(jobIDs, id)
		}
	}
	m.mu.RUnlock()

	parallel := len(jobIDs)

	if parallel <= 0 {
		return
	}

	m.opt.Parallel = parallel
	sem := make(chan struct{}, parallel)
	m.sem = sem

	for _, id := range jobIDs {
		if err := m.StartJob(ctx, id); err != nil {
			log.Printf("StartAllJobs: failed to start job %s: %v", id, err)
		}
	}
}

func (m *JobManager) PauseJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}
	if job.Status != StatusRunning {
		return fmt.Errorf("job %s is not running", jobID)
	}
	if job.CancelFunc != nil {
		job.CancelFunc()
	}

	return nil
}

func (m *JobManager) PauseAllJobs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, job := range m.jobs {
		if job.Status != StatusRunning {
			continue
		}
		if job.CancelFunc != nil {
			job.CancelFunc()
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

	if job.CancelFunc != nil {
		job.CancelFunc()
	}

	delete(m.jobs, jobID)
	m.mu.Unlock()

	if err := DeleteJobFromDisk(jobID); err != nil {
		return fmt.Errorf("failed to delete job from disk: %w", err)
	}

	return nil
}

func (m *JobManager) DeleteJobsByFilter(filter string) error {
	if filter == "" {
		filter = "all"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var remaining []*Job
	for _, job := range m.jobs {
		switch filter {
		case "completed":
			if job.Status == StatusCompleted {
				continue
			}
		case "all":
			continue
		}
		remaining = append(remaining, job)
	}

	if err := filesystem.WriteMetadataFile("jobs.json", remaining); err != nil {
		return err
	}

	m.jobs = make(map[string]*Job, len(remaining))
	for _, job := range remaining {
		m.jobs[job.ID] = job
	}

	m.urls = make(map[string]string)

	return nil
}
