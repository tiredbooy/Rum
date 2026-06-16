package download

import (
	"context"
	"fmt"
	"log"
	"mime"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gen2brain/beeep"
	"github.com/google/uuid"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
	"github.com/tiredbooy/Rum/backend/internal/pkg/format"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

type JobManager struct {
	mu             sync.RWMutex
	jobs           map[string]*Job
	urls           map[string]string
	sem            chan struct{}
	opt            *Options
	subMu          sync.RWMutex
	subscribers    map[string][]chan dto.ProgressUpdate
	allSubscribers []chan dto.ProgressUpdate
	sortBy         SortField
	batchCounters  map[string]int32
	batchMu        sync.Mutex
	config         config.Setting
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

	m := &JobManager{
		jobs:          make(map[string]*Job),
		urls:          make(map[string]string),
		sem:           make(chan struct{}, opt.Parallel),
		opt:           opt,
		subscribers:   make(map[string][]chan dto.ProgressUpdate),
		batchCounters: make(map[string]int32),
		config:        setting,
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
		return nil, fmt.Errorf("no URLs provided")
	}

	// 1. Filter already known URLs
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
	job.SetCancelFunc(cancel)

	select {
	case m.sem <- struct{}{}:
		log.Printf("StartJob: acquired semaphore for %s", jobID)
	case <-ctx.Done():
		return ctx.Err()
	}

	go m.runDownload(bgCtx, jobID, cancel)
	return nil
}

func (m *JobManager) runDownload(ctx context.Context, jobID string, cancel context.CancelFunc) {
	defer func() { <-m.sem }()
	defer cancel()

	m.mu.Lock()
	job := m.jobs[jobID]
	job.SetStatus(StatusRunning)
	m.mu.Unlock()

	var lastDownloaded int64
	var lastTime time.Time
	var smoothSpeed float64

	progressFn := func(downloaded, total int64) {
		now := time.Now()

		var instantSpeed float64
		if !lastTime.IsZero() {
			elapsed := now.Sub(lastTime).Seconds()
			if elapsed > 0 {
				instantSpeed = float64(downloaded-lastDownloaded) / elapsed
			}
		}

		lastDownloaded = downloaded
		lastTime = now

		const alpha = 0.3
		if smoothSpeed == 0 {
			smoothSpeed = instantSpeed
		} else {
			smoothSpeed = alpha*instantSpeed + (1-alpha)*smoothSpeed
		}

		var eta int64
		if smoothSpeed > 0 && total > 0 {
			remaining := total - downloaded
			if remaining > 0 {
				eta = int64(float64(remaining) / smoothSpeed)
			}
		}

		job.SetSpeed(smoothSpeed)

		update := dto.ProgressUpdate{
			JobID:      jobID,
			Downloaded: downloaded,
			TotalSize:  total,
			Speed:      smoothSpeed,
			Status:     string(job.Status),
			Progress:   progressPercent(downloaded, total),
			ETA:        eta,
		}
		m.publishProgress(update)
	}

	err := DownloadSegmented(ctx, *m.opt, job, progressFn)

	m.mu.Lock()
	if ctx.Err() == context.Canceled {
		job.SetStatus(StatusPaused)
	} else if err == nil {
		job.SetStatus(StatusCompleted)
		job.SetCompletedAt(time.Now())
	} else {
		log.Println("ERROR STARTING: ", err.Error())
		job.SetStatus(StatusError)
		job.Error = err
	}
	downloaded := job.GetDownloaded()
	totalSize := job.TotalSize
	finalStatus := string(job.Status)
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

	m.saveToDisk()

	m.onJobFinished(job)
}

func (m *JobManager) StartAllJobs(ctx context.Context) {
	batchID := fmt.Sprintf("batch_%d", time.Now().UnixNano())
	var eligibleJobs []*Job
	m.mu.RLock()

	for _, job := range m.jobs {
		if job.Status == StatusPending || job.Status == StatusPaused {
			eligibleJobs = append(eligibleJobs, job)
		}
	}
	m.mu.RUnlock()

	m.batchMu.Lock()
	m.batchCounters[batchID] = int32(len(eligibleJobs))
	m.batchMu.Unlock()

	sort.Slice(eligibleJobs, func(i, j int) bool {
		if m.sortBy == "created_at" {
			return utils.TimeCompare(eligibleJobs[i].CreatedAt, eligibleJobs[j].CreatedAt)
		}

		return eligibleJobs[i].FileName < eligibleJobs[j].FileName
	})

	for _, job := range eligibleJobs {
		job.BatchID = batchID
		m.StartJob(ctx, job.ID)
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
	if cancel := job.GetCancelFunc(); cancel != nil {
		cancel()
	}

	return nil
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

	if cancel := job.GetCancelFunc(); cancel != nil {
		cancel()
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
				continue // drop completed
			}
		case "error", "failed":
			if job.Status == StatusError {
				continue // drop errored/failed
			}
		case "all":
			continue // drop everything
		}
		remaining = append(remaining, job)
	}

	if err := filesystem.WriteMetadataFile("jobs.json", remaining); err != nil {
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
	var setting config.Setting
	setting.LoadSettingMetadata()
	m.config = setting

	if m.config.PostDownload.AutoOpenDir {
		filesystem.OpenFolder(job.OutputPath)
	}

	switch m.config.PostDownload.Action {
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

	if !m.config.Silent && job.GetStatus() == StatusCompleted {
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
