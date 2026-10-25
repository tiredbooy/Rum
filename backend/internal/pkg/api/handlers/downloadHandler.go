package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		Referer:      req.Referer,
		UserAgent:    req.UserAgent,
		SpeedLimit:   req.SpeedLimit,
		MaxRetries:   req.MaxRetries,
		DestPath:     req.DestPath,
		NightQueue:   req.NightQueue,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}

	if req.AutoStart {
		for _, job := range jobs {
			// A night-queue job outside its window waits for the night plan.
			if GlobalManager.NightHolds(job, time.Now()) {
				continue
			}
			// Honor a per-download scheduled start: a job with a future StartAt is
			// left pending for the schedule controller to start when due, even when
			// AutoStart is on (the default).
			if download.ShouldDeferAutoStart(job) {
				continue
			}
			go GlobalManager.StartJob(context.Background(), job.ID)
		}
	}

	jobInfos := make([]dto.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		info := dto.JobInfo{
			ID:     job.ID,
			URL:    job.URL,
			Status: string(job.Status),
		}
		if df, ok := download.AsDiskFull(job.GetError()); ok {
			info.ErrorCode, info.NeededBytes, info.FreeBytes = download.ErrorCodeDiskFull, df.NeededBytes, df.FreeBytes
		}
		jobInfos = append(jobInfos, info)
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

// RetryFailedDownloads restarts every failed download that is worth retrying now
// (skips disk-full while space is short, block pages unless overridden, and
// expired links).
//
//	POST /api/v1/downloads/retry-failed
//	body (optional): { "include_blocked": bool }
//	resp: { retried, skipped_disk_full, skipped_blocked, skipped_expired }
func RetryFailedDownloads(c *gin.Context) {
	var req struct {
		IncludeBlocked bool `json:"include_blocked"`
	}
	if c.Request.ContentLength != 0 {
		// An empty body (including unknown-length) means defaults.
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
			return
		}
	}
	c.JSON(http.StatusOK, GlobalManager.RetryFailed(c.Request.Context(), req.IncludeBlocked))
}

// DownloadAnyway restarts a download that stopped on a block page and allows the
// HTML response for that one run.
func DownloadAnyway(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	if err := GlobalManager.DownloadAnyway(c.Request.Context(), jobID); err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Download started."})
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

// RefreshDownloadURL gives a failed or paused download a fresh link (for expired
// URLs) and starts it again. Body: { "url": "...", "restart": false }.
//
// The new link is checked against the partial data on disk. On a match the
// download resumes from the partial (200, resumed:true). If it is a different
// file the partial is left untouched and the response is 409 remote_changed;
// resend with restart:true to discard it and download from zero.
func RefreshDownloadURL(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCreateBodyBytes)

	var req dto.UpdateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if fields := req.Validate(); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}

	res, err := GlobalManager.RefreshJobURL(context.Background(), jobID, strings.TrimSpace(req.URL), req.Restart)
	if errors.Is(err, download.ErrRemoteChanged) {
		writeError(c, http.StatusConflict, "remote_changed", "new link is a different file")
		return
	}
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": jobID, "resumed": res.Resumed, "restarted": res.Restarted})
}

// deleteResponse is shared by single and bulk delete: the hidden ids, the
// deadline, and the undo window so the UI can size its toast.
func deleteResponse(ids []string, at time.Time) gin.H {
	secs := int(time.Until(at).Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return gin.H{"ids": ids, "pending_delete_at": at.UTC().Format(time.RFC3339), "undo_seconds": secs}
}

// DeleteDownload hides a download and defers the removal so it can be undone
// (POST /downloads/:id/restore). ?delete_file=true also deletes the file or
// partial at the deadline; without it only the list entry goes ("remove from
// list") and the data stays on disk.
func DeleteDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	at, err := GlobalManager.SoftDeleteJob(jobID, c.Query("delete_file") == "true")
	if err != nil {
		writeServiceError(c, err)
		return
	}

	// NOTE: kept as 200 + JSON body (not a bodyless 204) because the frontend's
	// fetch wrapper calls res.json() unconditionally; an empty body would throw.
	c.JSON(http.StatusOK, deleteResponse([]string{jobID}, at))
}

// DeleteDownloads is the bulk form (?status=all|completed|error, plus
// ?delete_file=true). Undo it with POST /downloads/restore {ids}.
func DeleteDownloads(c *gin.Context) {
	filter := c.Query("status")

	ids, at, err := GlobalManager.SoftDeleteJobsByFilter(filter, c.Query("delete_file") == "true")
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if ids == nil {
		ids = []string{}
		at = time.Now()
	}
	c.JSON(http.StatusOK, deleteResponse(ids, at))
}

// RestoreDownload cancels a pending delete. 404 once the undo window is over.
func RestoreDownload(c *gin.Context) {
	jobID := c.Param("id")
	if err := GlobalManager.RestoreJob(jobID); err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restored": 1})
}

// RestoreDownloads undoes a bulk delete.
func RestoreDownloads(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "ids are required")
		return
	}
	c.JSON(http.StatusOK, gin.H{"restored": GlobalManager.RestoreJobs(req.IDs)})
}

func GetDownloadsStats(c *gin.Context) {
	c.JSON(http.StatusOK, GlobalManager.GetDashboardStats())
}

// VerifyDownload checks an (already finished) download's integrity and returns
// the engine VerifyResult. ?deep=true forces a network re-validation (re-HEAD to
// confirm the source still matches). It never mutates the file, so it is always
// safe to call. Errors map via the engine taxonomy: not-found -> 404,
// remote-changed -> 409, checksum/integrity -> 422.
func VerifyDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	deep := c.Query("deep") == "true" || c.Query("deep") == "1"

	res, err := GlobalManager.VerifyJob(c.Request.Context(), jobID, deep)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// RepairDownload re-fetches the bad/missing segments of a download and re-verifies
// it. The optional body/query `segments` selects which segments to repair; when
// absent the handler verifies first to localize the damage, then repairs those.
// It is idempotent and safe on an already-good completed job (it returns a clean
// verify result without re-downloading). Errors map via the engine taxonomy:
// not-found -> 404, conflict (still downloading) -> 409, remote-changed -> 409,
// checksum/integrity -> 422.
func RepairDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "job id is required")
		return
	}

	// Optional segments selector. Accept either a JSON body { "segments": [..] }
	// or a comma-separated ?segments=0,2,5 query param. nil means "verify first to
	// find bad segments, then repair them".
	segments := parseRepairSegments(c)

	res, err := GlobalManager.RepairJob(c.Request.Context(), jobID, segments)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// parseRepairSegments extracts the optional segment indices to repair from the
// request, preferring a JSON body and falling back to a ?segments= query param.
// It returns nil when none are supplied (the handler then verifies to localize
// damage), and a non-nil (possibly empty) slice when an explicit selection is
// given. Invalid/negative entries are dropped.
func parseRepairSegments(c *gin.Context) []int {
	// Try a JSON body first (best-effort; an empty/absent body is fine).
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCreateBodyBytes)
		var body struct {
			Segments []int `json:"segments"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.Segments != nil {
			return sanitizeSegments(body.Segments)
		}
	}

	// Fall back to a comma-separated query param.
	if q := c.Query("segments"); q != "" {
		var out []int
		for _, part := range strings.Split(q, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if n, err := strconv.Atoi(part); err == nil && n >= 0 {
				out = append(out, n)
			}
		}
		return sanitizeSegments(out)
	}

	return nil
}

// sanitizeSegments drops negative indices and returns the cleaned slice. A
// non-nil but empty result means "explicit empty selection" (repair nothing,
// just re-verify), distinct from nil ("figure out what's bad").
func sanitizeSegments(in []int) []int {
	out := make([]int, 0, len(in))
	for _, n := range in {
		if n >= 0 {
			out = append(out, n)
		}
	}
	return out
}

// toDownloadResponse builds the single-job response (used by GET /downloads/:id).
func toDownloadResponse(job *download.Job) dto.DownloadResponse {
	progress := 0
	if job.TotalSize > 0 {
		progress = int(float64(job.Downloaded) / float64(job.TotalSize) * 100)
	}
	resp := dto.DownloadResponse{
		ID:         job.ID,
		URL:        job.URL,
		Filename:   job.FileName,
		Status:     string(job.Status),
		Progress:   progress,
		Downloaded: job.Downloaded,
		TotalSize:  job.TotalSize,
		Speed:      job.GetSpeed(),
		Category:   job.GetCategory(),
		OutputPath: job.GetOutputPath(),
	}
	resp.NightQueue, resp.NightPaused = job.GetNightQueue(), job.GetNightPaused()
	setNightLimit(&resp, job)
	if e := job.GetError(); e != nil {
		resp.Error = e.Error()
		resp.ErrorCode, resp.Retryable = job.GetErrorCode()
		addDiskFull(&resp, e)
	}
	return resp
}

// addDiskFull copies the disk-full details of err, when it is one, into resp.
func addDiskFull(resp *dto.DownloadResponse, err error) {
	if df, ok := download.AsDiskFull(err); ok {
		resp.ErrorCode = download.ErrorCodeDiskFull
		resp.NeededBytes, resp.FreeBytes, resp.DiskPath = df.NeededBytes, df.FreeBytes, df.Path
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
		resp.ErrorCode, resp.Retryable = job.GetErrorCode()
		addDiskFull(&resp, job.Error)
	}
	if job.GetStallCode() != "" && resp.Error == "" {
		resp.ErrorCode = download.ErrorCodeStalled
		resp.Error = "stalled, reconnecting"
	}
	resp.FinalHost = job.FinalHost
	resp.NightQueue, resp.NightPaused = job.GetNightQueue(), job.GetNightPaused()
	if resp.ErrorCode == "" {
		setNightLimit(&resp, job)
	}
	return resp
}

// setNightLimit marks a night job held by the spent daily cap ("night_limit").
func setNightLimit(resp *dto.DownloadResponse, job *download.Job) {
	if GlobalManager != nil {
		resp.ErrorCode = GlobalManager.NightStatusCode(job)
	}
}
