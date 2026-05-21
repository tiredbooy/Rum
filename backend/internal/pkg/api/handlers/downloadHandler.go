package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	"github.com/tiredbooy/Rum/backend/internal/pkg/utils"
)

var GlobalManager *download.JobManager

func InitAPI(opt *download.Options) {
	opt.Downloader = download.NewDownloader("", "") 
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
	statusFilter := c.Query("status")

	var result []dto.DownloadResponse
	for _, job := range GlobalManager.GetAllJobs() {
		if statusFilter == "" || statusFilter == "all" {
			// include all
		} else if job.Status != statusFilter {
			continue
		}

		j := dto.DownloadResponse{
			ID:          job.ID,
			URL:         job.URL,
			Filename:    job.FileName,
			Progress:    utils.GetProgress(job.Downloaded, job.TotalSize),
			Status:      job.Status,
			Downloaded:  job.Downloaded,
			TotalSize:   job.TotalSize,
			Speed:       job.Speed,
			Remaining:   int64(job.RemainingTime),
			CreatedAt:   job.CreatedAt.String(),
			CompletedAt: job.CompletedAt,
		}
		result = append(result, j)
	}

	if len(result) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No Job Found"})
		return
	}
	c.JSON(http.StatusOK, result)
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

	c.JSON(http.StatusAccepted, gin.H{"message": "Job Started Successfully."})
}

func StartDownloads(c *gin.Context) {
	go GlobalManager.StartAllJobs(context.Background())
	c.JSON(http.StatusAccepted, gin.H{"message": "Starting all pending/paused jobs"})
}

func PauseDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Job ID, please provide valid jobID"})
		return
	}

	err := GlobalManager.PauseJob(jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Downlaod Paused."})
}

func PauseDownloads(c *gin.Context) {
	err := GlobalManager.PauseAllJobs()
	if err != nil {
		log.Println("ERROR: ", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "All Downlaods Paused."})
}


func ResumeDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Job ID, please provide valid jobID"})
		return
	}

	err := GlobalManager.StartJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "Download Resumed."})
}

func DeleteDownload(c *gin.Context) {
	jobID := c.Param("id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Job ID, please provide valid jobID"})
		return
	}

	err := GlobalManager.DeleteJob(jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, gin.H{"message": "Download Deleted."})
}

func DeleteDownloads(c *gin.Context) {
	filter := c.Query("status")
	
	err := GlobalManager.DeleteJobsByFilter(filter) 
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete jobs"})
		return
	}

	c.JSON(http.StatusNoContent, gin.H{"message": "Jobs deleted successfully."})
}