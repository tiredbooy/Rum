package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	"github.com/tiredbooy/Rum/backend/internal/pkg/ytdlp"
)

const maxVideoBodyBytes = 16 << 10

// ytdlpToolMu serialises install/update so two clicks cannot race on the file.
var ytdlpToolMu sync.Mutex

// ytdlpStatusResponse tells the UI whether video downloads can run now.
type ytdlpStatusResponse struct {
	Found   bool   `json:"found"`
	Source  string `json:"source,omitempty"` // configured | path | managed
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	FFmpeg  bool   `json:"ffmpeg"`
	// CanInstall is false on systems without an official build.
	CanInstall bool `json:"can_install"`
}

func toolStatus(ctx context.Context) ytdlpStatusResponse {
	configured, _ := GlobalManager.YtdlpSettings()
	st := ytdlpStatusResponse{FFmpeg: ytdlp.HasFFmpeg(), CanInstall: ytdlp.Installer{}.Supported()}
	if f, err := ytdlp.Locate(configured); err == nil {
		st.Found, st.Source, st.Path = true, f.Source, f.Path
		st.Version = ytdlp.Version(ctx, f.Path)
	}
	return st
}

// requireJSON refuses bodies a web page could send cross-site without a CORS
// preflight (simple form posts), since these routes start local processes.
func requireJSON(c *gin.Context) bool {
	if c.ContentType() != "application/json" {
		writeError(c, http.StatusUnsupportedMediaType, dto.CodeUnsupported, "send application/json")
		return false
	}
	return true
}

// writeYtdlpError maps a yt-dlp failure to a status and the envelope. Its code
// is the same machine code a failed job would carry.
func writeYtdlpError(c *gin.Context, err error) {
	var ye *ytdlp.Error
	if !errors.As(err, &ye) {
		if errors.Is(err, context.Canceled) {
			writeError(c, http.StatusRequestTimeout, dto.CodeInternal, "cancelled")
			return
		}
		writeServiceError(c, err)
		return
	}
	status := http.StatusBadGateway
	switch ye.Code {
	case ytdlp.CodeMissing:
		status = http.StatusFailedDependency
	case ytdlp.CodeVideoUnavailable:
		status = http.StatusUnprocessableEntity
	case "timeout":
		status = http.StatusGatewayTimeout
	}
	writeError(c, status, ye.Code, ye.Message)
}

// GetYtdlpStatus reports where yt-dlp was found (if at all) and whether ffmpeg exists.
func GetYtdlpStatus(c *gin.Context) {
	c.JSON(http.StatusOK, toolStatus(c.Request.Context()))
}

func installer() ytdlp.Installer {
	return ytdlp.Installer{Client: download.NewDownloader("Rum", "").Client}
}

// InstallYtdlp downloads the official yt-dlp release (through Rum's proxy
// settings), checks it against the release checksums and stores it.
func InstallYtdlp(c *gin.Context) {
	if !requireJSON(c) {
		return
	}
	ytdlpToolMu.Lock()
	defer ytdlpToolMu.Unlock()
	if _, err := installer().Install(c.Request.Context()); err != nil {
		writeYtdlpError(c, err)
		return
	}
	c.JSON(http.StatusOK, toolStatus(c.Request.Context()))
}

// UpdateYtdlp updates the yt-dlp Rum would use.
func UpdateYtdlp(c *gin.Context) {
	if !requireJSON(c) {
		return
	}
	configured, _ := GlobalManager.YtdlpSettings()
	f, err := ytdlp.Locate(configured)
	if err != nil {
		writeYtdlpError(c, err)
		return
	}
	ytdlpToolMu.Lock()
	defer ytdlpToolMu.Unlock()
	if err := ytdlp.Update(c.Request.Context(), f, installer()); err != nil {
		writeYtdlpError(c, err)
		return
	}
	c.JSON(http.StatusOK, toolStatus(c.Request.Context()))
}

type videoInfoRequest struct {
	URL string `json:"url"`
}

type videoInfoResponse struct {
	Title    string         `json:"title"`
	Uploader string         `json:"uploader,omitempty"`
	Duration int            `json:"duration,omitempty"` // seconds
	FFmpeg   bool           `json:"ffmpeg"`
	Options  []ytdlp.Option `json:"options"`
}

// infoCache keeps recent format listings so picking a format and pressing
// download does not run yt-dlp -J a second time.
type infoCache struct {
	mu sync.Mutex
	m  map[string]cachedInfo
}

type cachedInfo struct {
	info *ytdlp.Info
	at   time.Time
}

const (
	infoCacheTTL = 10 * time.Minute
	infoCacheMax = 32
)

var videoInfos = &infoCache{m: map[string]cachedInfo{}}

func (c *infoCache) get(url string) *ytdlp.Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[url]
	if !ok || time.Since(e.at) > infoCacheTTL {
		delete(c.m, url)
		return nil
	}
	return e.info
}

func (c *infoCache) put(url string, info *ytdlp.Info) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= infoCacheMax {
		oldest, at := "", time.Now()
		for k, v := range c.m {
			if v.at.Before(at) {
				oldest, at = k, v.at
			}
		}
		delete(c.m, oldest)
	}
	c.m[url] = cachedInfo{info: info, at: time.Now()}
}

// loadVideoInfo returns the page's formats, from cache or by running yt-dlp.
func loadVideoInfo(ctx context.Context, link string, fresh bool) (*ytdlp.Info, error) {
	if !fresh {
		if info := videoInfos.get(link); info != nil {
			return info, nil
		}
	}
	configured, proxy := GlobalManager.YtdlpSettings()
	f, err := ytdlp.Locate(configured)
	if err != nil {
		return nil, err
	}
	info, err := ytdlp.FetchInfo(ctx, f.Path, link, proxy)
	if err != nil {
		return nil, err
	}
	videoInfos.put(link, info)
	return info, nil
}

func bindVideoURL(c *gin.Context, into interface{ urlField() *string }) (string, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxVideoBodyBytes)
	if !requireJSON(c) {
		return "", false
	}
	if err := c.ShouldBindJSON(into); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return "", false
	}
	link := strings.TrimSpace(*into.urlField())
	if err := ytdlp.ValidateURL(link); err != nil {
		writeFieldErrors(c, "validation failed", map[string]string{"url": "must be an http(s) link"})
		return "", false
	}
	return link, true
}

func (r *videoInfoRequest) urlField() *string { return &r.URL }

// GetVideoInfo lists the formats of a page: POST {url} -> title + picker options.
func GetVideoInfo(c *gin.Context) {
	var req videoInfoRequest
	link, ok := bindVideoURL(c, &req)
	if !ok {
		return
	}
	info, err := loadVideoInfo(c.Request.Context(), link, true)
	if err != nil {
		writeYtdlpError(c, err)
		return
	}
	ff := ytdlp.HasFFmpeg()
	c.JSON(http.StatusOK, videoInfoResponse{
		Title:    info.Title,
		Uploader: info.Uploader,
		Duration: int(info.Duration),
		FFmpeg:   ff,
		Options:  ytdlp.Options(info, ff),
	})
}

type videoDownloadRequest struct {
	URL        string `json:"url"`
	Format     string `json:"format"`
	DestPath   string `json:"dest_path"`
	SpeedLimit int    `json:"speed_limit"`
	AutoStart  *bool  `json:"auto_start,omitempty"` // default true
}

func (r *videoDownloadRequest) urlField() *string { return &r.URL }

// CreateVideoDownload queues the chosen format of a page as a download job.
func CreateVideoDownload(c *gin.Context) {
	var req videoDownloadRequest
	link, ok := bindVideoURL(c, &req)
	if !ok {
		return
	}
	info, err := loadVideoInfo(c.Request.Context(), link, false)
	if err != nil {
		writeYtdlpError(c, err)
		return
	}
	ff := ytdlp.HasFFmpeg()
	offered := false
	for _, o := range ytdlp.Options(info, ff) {
		offered = offered || o.Key == req.Format
	}
	if !offered {
		writeFieldErrors(c, "validation failed", map[string]string{"format": "not available for this video"})
		return
	}
	job, err := GlobalManager.CreateVideoJob(download.VideoJobRequest{
		URL: link, Title: info.Title, Format: req.Format,
		EstSize: ytdlp.EstimateSize(info, req.Format, ff), DestPath: req.DestPath, SpeedLimit: req.SpeedLimit,
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	if req.AutoStart == nil || *req.AutoStart {
		go GlobalManager.StartJob(context.Background(), job.ID)
	}
	c.JSON(http.StatusCreated, gin.H{"jobs": []dto.JobInfo{{ID: job.ID, URL: job.URL, Status: job.GetStatus()}}})
}

// ExtVideo is the browser extension's "Download video with Rum" menu: it takes
// a page URL and queues the best format right away.
func ExtVideo(c *gin.Context) {
	var req videoInfoRequest
	link, ok := bindVideoURL(c, &req)
	if !ok {
		return
	}
	if id, known := GlobalManager.GetJobIDByURL(link); known {
		if job, ok := GlobalManager.GetJob(id); ok && isLiveStatus(job.GetStatus()) {
			c.JSON(http.StatusOK, gin.H{"id": id, "duplicate": true})
			return
		}
		writeError(c, http.StatusConflict, dto.CodeConflict, "Rum already has this link")
		return
	}
	info, err := loadVideoInfo(c.Request.Context(), link, false)
	if err != nil {
		writeYtdlpError(c, err)
		return
	}
	ff := ytdlp.HasFFmpeg()
	job, err := GlobalManager.CreateVideoJob(download.VideoJobRequest{
		URL: link, Title: info.Title, Format: "best", EstSize: ytdlp.EstimateSize(info, "best", ff),
	})
	if err != nil {
		writeServiceError(c, err)
		return
	}
	go GlobalManager.StartJob(context.Background(), job.ID)
	c.JSON(http.StatusCreated, gin.H{"id": job.ID, "status": job.GetStatus(), "title": info.Title})
}
