package download

import (
	"context"
	"fmt"
	"log"
	"mime"
	"sync"

	"github.com/google/uuid"
	"github.com/tiredbooy/Rum/internal/pkg/format"
	"github.com/tiredbooy/Rum/internal/pkg/utils"
)

type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*Job
	urls map[string]string
	sem  chan struct{}
	opt  *Options
}

func NewJobManager(opt *Options) *JobManager {
	if opt == nil {
		opt = &Options{Parallel: 1}
	}
	if opt.Parallel < 1 {
		opt.Parallel = 1
	}

	m := &JobManager{
		jobs: make(map[string]*Job),
		urls: make(map[string]string),
		sem:  make(chan struct{}, opt.Parallel),
		opt:  opt,
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
			log.Println("Reached head req")
			info, err := downloader.HeadWithFallback(u)
			log.Println("done head req")
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
	return jobs
}

func (m *JobManager) StartJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	job, exists := m.jobs[jobID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("Job %s not found", jobID)
	}
	if job.Status != StatusPending && job.Status != StatusPaused {
		m.mu.Unlock()
		return fmt.Errorf("job %s is already %s", jobID, job.Status)
	}
	job.SetStatus(StatusPending)
	m.mu.Unlock()

	select {
	case m.sem <- struct{}{}:
		// acquired
	case <-ctx.Done():
		return ctx.Err()
	}

	go func() {
		defer func() { <-m.sem }() // release worker

		// Update status to running
		m.mu.Lock()
		job.SetStatus(StatusRunning)
		m.mu.Unlock()

		err := DownloadSingleFile(ctx, *m.opt, job, nil)

		m.mu.Lock()
		defer m.mu.Unlock()
		if ctx.Err() == context.Canceled {
			job.SetStatus(StatusPaused)
		} else if err == nil {
			job.SetStatus(StatusCompleted)
		} else {
			job.SetStatus(StatusError)
			job.Error = err
		}
		m.saveToDisk()
	}()
	return nil
}

func (m *JobManager) PauseJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found")
	}
	if job.CancelFunc != nil {
		job.CancelFunc()
		job.SetStatus(StatusPaused)
		m.saveToDisk()
		return nil
	}
	return fmt.Errorf("job %s has no active download", jobID)
}
