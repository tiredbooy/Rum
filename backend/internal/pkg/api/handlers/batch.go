package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

// maxBatchBodyBytes caps the batch request body. Batches can contain many URLs,
// so allow a larger body than a single create while still bounding memory.
const maxBatchBodyBytes = 4 << 20 // 4 MiB

// CreateBatch creates many downloads at once (clipboard / multi-URL add).
//
//	POST /api/v1/downloads/batch
//	body: { "urls": [...], "options"?: { ... } }
//	resp: { "created": DownloadResponse[], "errors": [{url, error}] }
//
// Each URL is validated independently: malformed/unsupported URLs are returned in
// `errors` (with a stable code-derived message) while valid ones are created. The
// response is 200 even with partial errors so the client can show a per-URL
// summary.
func CreateBatch(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBatchBodyBytes)

	var req dto.BatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if len(req.URLs) == 0 {
		writeFieldErrors(c, "validation failed", map[string]string{"urls": "at least one URL is required"})
		return
	}

	// Parse shared options once.
	var (
		startAt time.Time
		opts    download.JobCreateOptions
	)
	if req.Options != nil {
		if s := strings.TrimSpace(req.Options.StartAt); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				writeFieldErrors(c, "validation failed", map[string]string{"options.start_at": "must be an RFC3339 timestamp"})
				return
			}
			startAt = t
		}
		if strings.TrimSpace(req.Options.Checksum) != "" {
			switch strings.ToLower(strings.TrimSpace(req.Options.ChecksumAlgo)) {
			case "sha256", "md5":
			default:
				writeFieldErrors(c, "validation failed", map[string]string{"options.checksum_algo": `must be "sha256" or "md5"`})
				return
			}
		}
		opts = download.JobCreateOptions{
			Checksum:     req.Options.Checksum,
			ChecksumAlgo: req.Options.ChecksumAlgo,
			Category:     req.Options.Category,
			StartAt:      startAt,
		}
	}

	// Validate each URL up front so we can report per-URL errors. Valid URLs are
	// passed to the manager (which also dedupes already-known URLs).
	resp := dto.BatchCreateResultResponse{
		Created: []dto.DownloadResponse{},
		Errors:  []dto.BatchURLError{},
	}
	var valid []string
	for _, raw := range req.URLs {
		u := strings.TrimSpace(raw)
		if err := download.ValidateURL(u); err != nil {
			resp.Errors = append(resp.Errors, dto.BatchURLError{
				URL:   raw,
				Error: batchErrMessage(err),
			})
			continue
		}
		valid = append(valid, u)
	}

	if len(valid) > 0 {
		jobs, err := GlobalManager.CreateJobsFromURLsWithOptions(valid, opts)
		if err != nil {
			// A whole-batch failure (e.g. all HEADs failed). Surface it as a single
			// classified error rather than per-URL.
			writeServiceError(c, err)
			return
		}
		for _, job := range jobs {
			resp.Created = append(resp.Created, toDownloadResponse(job))
		}
		if req.Options != nil && req.Options.AutoStart {
			for _, job := range jobs {
				// Honor a per-download scheduled start (see ShouldDeferAutoStart):
				// future-dated jobs stay pending for the schedule controller.
				if download.ShouldDeferAutoStart(job) {
					continue
				}
				go GlobalManager.StartJob(c.Request.Context(), job.ID)
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

// batchErrMessage maps an engine URL error to a stable, client-safe message via
// the shared Classify codes (validation_error / unsupported / ...).
func batchErrMessage(err error) string {
	code, _ := download.Classify(err)
	switch code {
	case dto.CodeValidation:
		return "invalid url"
	case dto.CodeUnsupported:
		return "unsupported url scheme"
	default:
		return "invalid url"
	}
}
