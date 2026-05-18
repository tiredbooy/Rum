package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

var GlobalManager *download.JobManager

func InitAPI(opt *download.Options) {
	GlobalManager = download.NewJobManager(opt)
}

func CreateDownload(c *gin.Context) {
	var req dto.CreateDownloadRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jobs, err := GlobalManager.CreateJobsFromURLs(req.URLs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, job := range jobs {
		if req.AutoStart {
			go GlobalManager.StartJob(c.Request.Context(), job.ID)
		}
	}

	var jobInfos []dto.JobInfo
	for _, job := range jobs {
		jobInfos = append(jobInfos, dto.JobInfo{
			ID:     job.ID,
			URL:    job.URL,
			Status: string(job.Status),
		})
	}
	c.JSON(http.StatusCreated, gin.H{"jobs": jobInfos})
}

func GetDownloadStatus(c *gin.Context) {
	jobID := c.Param("id")
	job, ok := GlobalManager.GetJob(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

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
		// CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	c.JSON(http.StatusOK, resp)
}

func GetAllJobs(c *gin.Context) {
	var jobs []dto.DownloadResponse

	foundJob := GlobalManager.GetAllJobs()

	for _, job := range foundJob {

		j := dto.DownloadResponse{
			ID:         job.ID,
			URL:        job.URL,
			Filename:   job.FileName,
			Progress:   (int(job.Downloaded) / int(job.TotalSize)) * 100,
			Status:     job.Status,
			Downloaded: job.Downloaded,
			TotalSize:  job.TotalSize,
			Speed:      job.Speed,
			Remaining:  int64(job.RemainingTime),
			// Error:       job.Error.Error(),
			CreatedAt:   job.CreatedAt,
			CompletedAt: job.CompletedAt,
		}

		jobs = append(jobs, j)
	}

	if len(jobs) <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No Job Found"})
		return
	}

	c.JSON(http.StatusOK, jobs)
}

func StartDownload(c *gin.Context) {
	jobID := c.Param("id")

	jobExists := GlobalManager.CheckJobExists(c.Request.Context(), jobID)
	if !jobExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job does not exists"})
		return
	}

	err := GlobalManager.StartJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start download %s: ", err.Error())})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job Started Successfully."})
}

func StartDownloads(c *gin.Context) {
	go GlobalManager.StartAllJobs(c.Request.Context())
	c.JSON(http.StatusAccepted, gin.H{"message": "Starting all pending/paused jobs"})
}
