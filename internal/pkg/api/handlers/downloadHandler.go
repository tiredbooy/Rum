package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"swiftget.com/internal/pkg/api/dto"
	"swiftget.com/internal/pkg/download"
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
		go GlobalManager.StartJob(c.Request.Context(), job.ID)
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
