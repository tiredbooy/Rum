package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

// maxCreateBodyBytes caps the request body for create-download so a client
// can't exhaust memory by streaming an enormous JSON body (go-api-design).
const maxCreateBodyBytes = 1 << 20 // 1 MiB

var GlobalManager *download.JobManager

func InitAPI(opt *download.Options) {
	opt.Downloader = download.NewDownloader("", "")
	GlobalManager = download.NewJobManager(opt)
}

func CreateDownload(c *gin.Context) {
	// Cap body size before decoding.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCreateBodyBytes)

	var req dto.CreateDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}

	if fields := req.Validate(); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}

	jobs, err := GlobalManager.CreateJobsFromURLsWithOptions(req.URLs, download.JobCreateOptions{
		Checksum:     req.Checksum,
		ChecksumAlgo: req.ChecksumAlgo,
		Category:     req.Category,
		StartAt:      req.ParsedStartAt(),
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}

	if req.AutoStart {
		for _, job := range jobs {
			go GlobalManager.StartJob(context.Background(), job.ID)
		}
	}

	jobInfos := make([]dto.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		jobInfos = append(jobInfos, dto.JobInfo{
			ID:     job.ID,
			URL:    job.URL,
			Status: string(job.Status),
		})
	}
	// Backward-compatible success shape: { "jobs": [...] }.
	c.JSON(http.StatusCreated, gin.H{"jobs": jobInfos})
}

func GetDownloadStatus(c *gin.Context) {
	jobID := c.Param("id")
	job, ok := GlobalManager.GetJob(jobID)
	if !ok {
		writeError(c, http.StatusNotFound, dto.CodeNotFound, "job not found")
		return
	}

	c.JSON(http.StatusOK, toDownloadResponse(job))
}

// GetAllJobs returns downloads, optionally filtered by ?status=.
//
// Backward-compatible by default: returns a bare JSON array (what the frontend
// expects). Pagination is OPT-IN: it is only activated when the client passes
// ?limit= or ?offset=, in which case the response is the additive ListPage
// envelope. This avoids breaking the existing TanStack Query contract.
func GetAllJobs(c *gin.Context) {
	statusFilter := c.Query("status")

	all := GlobalManager.GetAllJobs()
	filtered := make([]dto.DownloadResponse, 0, len(all))
	for _, job := range all {
		if statusFilter != "" && statusFilter != "all" && job.Status != statusFilter {
			continue
		}
		filtered = append(filtered, toDownloadResponseFull(job))
	}

	_, hasLimit := c.GetQuery("limit")
	_, hasOffset := c.GetQuery("offset")
	if hasLimit || hasOffset {
		c.JSON(http.StatusOK, paginate(filtered, c))
		return
	}

	// Legacy shape preserved exactly: bare array, or a message object when empty.
	if len(filtered) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No Job Found"})
		return
	}
	c.JSON(http.StatusOK, filtered)
}

func paginate(items []dto.DownloadResponse, c *gin.Context) dto.ListPage {
	limit := dto.ClampLimit(atoiDefault(c.Query("limit"), dto.DefaultPageSize))
	offset := atoiDefault(c.Query("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	total := len(items)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	page := dto.ListPage{
		Items:  items[start:end],
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
	if page.Items == nil {
		page.Items = []dto.DownloadResponse{}
	}
	if end < total {
		next := end
		page.NextOffset = &next
	}
	return page
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func StartDownload(c *gin.Context) {
	jobID := c.Param("id")

	if !GlobalManager.CheckJobExists(c.Request.Context(), jobID) {
		writeError(c, http.StatusNotFound, dto.CodeNotFound, "job not found")
		return
	}

	if err := GlobalManager.StartJob(c.Request.Context(), jobID); err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Job Started Successfully."})
}

func StartDownloads(c *gin.Context) {
	go GlobalManager.StartAllJobs(context.Background())
	c.JSON(http.StatusAccepted, gin.H{"message": "Starting all pending/paused jobs"})
}

func PauseDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	if err := GlobalManager.PauseJob(jobID); err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Downlaod Paused."})
}

func PauseDownloads(c *gin.Context) {
	if err := GlobalManager.PauseAllJobs(); err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "All Downlaods Paused."})
}

func ResumeDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	if err := GlobalManager.StartJob(c.Request.Context(), jobID); err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Download Resumed."})
}

// RetryDownload re-queues a failed/cancelled download. The engine decides
// whether to resume a partial or restart from scratch; this handler only maps
// the manager result onto the standard envelope.
//
// Wiring note: depends on GlobalManager.RetryJob(ctx, id) which the orchestrator
// adds to manager.go (func (m *JobManager) RetryJob(ctx context.Context, id string) error).
func RetryDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	if err := GlobalManager.RetryJob(c.Request.Context(), jobID); err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Download retry started."})
}

// SetDownloadPriority changes the scheduling priority of a single download.
// Body: { "priority": "low" | "normal" | "high" }.
//
// Wiring note: depends on GlobalManager.SetJobPriority(id, priority) which the
// orchestrator adds to manager.go (func (m *JobManager) SetJobPriority(id, priority string) error).
func SetDownloadPriority(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCreateBodyBytes)

	var req dto.SetPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if fields := req.Validate(); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}

	if err := GlobalManager.SetJobPriority(jobID, req.Priority); err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": jobID, "priority": req.Priority})
}

func DeleteDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	if err := GlobalManager.DeleteJob(jobID); err != nil {
		writeServiceError(c, err)
		return
	}

	// NOTE: kept as 200 + JSON body (not a bodyless 204) because the frontend's
	// fetch wrapper calls res.json() unconditionally; an empty body would throw.
	c.JSON(http.StatusOK, gin.H{"message": "Download Deleted."})
}

func DeleteDownloads(c *gin.Context) {
	filter := c.Query("status")

	if err := GlobalManager.DeleteJobsByFilter(filter); err != nil {
		writeServiceError(c, err)
		return
	}

	// Kept as 200 + JSON body for the same reason as DeleteDownload (frontend
	// parses the body). The DELETE /downloads frontend call also tolerates a
	// non-array object, returning [].
	c.JSON(http.StatusOK, gin.H{"message": "Jobs deleted successfully."})
}

func GetDownloadsStats(c *gin.Context) {
	c.JSON(http.StatusOK, GlobalManager.GetDashboardStats())
}

// toDownloadResponse builds the single-job response (used by GET /downloads/:id).
func toDownloadResponse(job *download.Job) dto.DownloadResponse {
	progress := 0
	if job.TotalSize > 0 {
		progress = int(float64(job.Downloaded) / float64(job.TotalSize) * 100)
	}
	return dto.DownloadResponse{
		ID:         job.ID,
		URL:        job.URL,
		Filename:   job.FileName,
		Status:     string(job.Status),
		Progress:   progress,
		Downloaded: job.Downloaded,
		TotalSize:  job.TotalSize,
		Speed:      job.GetSpeed(),
		Category:   job.GetCategory(),
	}
}

// toDownloadResponseFull builds the list response (used by GET /downloads).
func toDownloadResponseFull(job *download.Job) dto.DownloadResponse {
	resp := dto.DownloadResponse{
		ID:          job.ID,
		URL:         job.URL,
		Filename:    job.FileName,
		Progress:    utils.GetProgress(job.Downloaded, job.TotalSize),
		Status:      job.Status,
		Downloaded:  job.Downloaded,
		TotalSize:   job.TotalSize,
		Speed:       job.Speed,
		Remaining:   int64(job.RemainingTime),
		Category:    job.GetCategory(),
		CreatedAt:   job.CreatedAt.String(),
		CompletedAt: job.CompletedAt.String(),
	}
	if job.Error != nil {
		resp.Error = job.Error.Error()
	}
	return resp
}
