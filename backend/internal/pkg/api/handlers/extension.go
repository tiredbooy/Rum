package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

// maxExtBodyBytes bounds /ext/add: a 16 KiB cookie plus the other fields.
const maxExtBodyBytes = 64 << 10

// ExtPing lets the extension check that Rum is running and the token is right
// (the auth middleware has already verified it by the time this runs).
func ExtPing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "app": "rum", "version": config.Load().Version})
}

// ExtAdd creates one download from the browser. The 2xx answer is the extension's
// cue to cancel its own download, so it is sent only once Rum holds the job (or
// already had it). Anything else leaves the browser download untouched.
func ExtAdd(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxExtBodyBytes)

	var req dto.ExtAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if fields := req.Validate(); fields != nil {
		writeFieldErrors(c, "validation failed", fields)
		return
	}
	link := strings.TrimSpace(req.URL)

	if id, known := GlobalManager.GetJobIDByURL(link); known {
		c.JSON(http.StatusOK, gin.H{"id": id, "duplicate": true})
		return
	}

	jobs, err := GlobalManager.CreateJobsFromURLsWithOptions([]string{link}, download.JobCreateOptions{
		Referer:   req.Referer,
		UserAgent: req.UserAgent,
		Cookie:    req.Cookie,
		FileName:  req.Filename,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if len(jobs) == 0 {
		// Nothing was created: the probe failed or the conflict policy skipped it.
		// 502 tells the extension to let the browser download instead.
		log.Printf("[request_id=%s] ext add: Rum could not take the link", c.GetString("request_id"))
		writeError(c, http.StatusBadGateway, dto.CodeInternal, "Rum could not reach the file")
		return
	}
	job := jobs[0]
	if req.StartNow() && !download.ShouldDeferAutoStart(job) {
		go GlobalManager.StartJob(context.Background(), job.ID)
	}
	c.JSON(http.StatusCreated, gin.H{"id": job.ID, "status": string(job.Status)})
}

// GetExtensionToken returns the pairing token for the Settings screen, creating
// it on first use. CORS keeps other websites from reading it.
func GetExtensionToken(c *gin.Context) {
	tok, err := config.ExtensionToken()
	if err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "could not read pairing token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tok})
}

// RegenerateExtensionToken replaces the token; every paired extension must be
// given the new one. It insists on a JSON content type, which a cross-site form
// cannot send without a CORS preflight, so a web page cannot trigger it.
func RegenerateExtensionToken(c *gin.Context) {
	if c.ContentType() != "application/json" {
		writeError(c, http.StatusUnsupportedMediaType, dto.CodeUnsupported, "send application/json")
		return
	}
	tok, err := config.RegenerateExtensionToken()
	if err != nil {
		writeError(c, http.StatusInternalServerError, dto.CodeInternal, "could not create pairing token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tok})
}

var (
	showWindowMu   sync.RWMutex
	showWindowHook func()
)

// SetShowWindowHook registers how to bring the desktop window forward. The
// desktop shell sets it; a headless build leaves it nil.
func SetShowWindowHook(f func()) {
	showWindowMu.Lock()
	showWindowHook = f
	showWindowMu.Unlock()
}

// ExtOpen brings the Rum window to the front (the popup's "Open Rum").
func ExtOpen(c *gin.Context) {
	showWindowMu.RLock()
	f := showWindowHook
	showWindowMu.RUnlock()
	if f == nil {
		writeError(c, http.StatusNotImplemented, dto.CodeUnsupported, "no window to open")
		return
	}
	f()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
