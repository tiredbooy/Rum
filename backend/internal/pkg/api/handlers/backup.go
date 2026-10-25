package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
)

// maxImportBodyBytes caps an imported list (tens of thousands of links fit).
const maxImportBodyBytes = 16 << 20 // 16 MiB

// ExportDownloads returns the download list as a JSON file.
//
//	GET /api/v1/downloads/export
func ExportDownloads(c *gin.Context) {
	c.Header("Content-Disposition", `attachment; filename="rum-downloads.json"`)
	c.JSON(http.StatusOK, GlobalManager.ExportDownloads())
}

// ImportDownloads adds an exported list as paused downloads.
//
//	POST /api/v1/downloads/import   body: the export file
//	resp: { added, skipped, invalid }
func ImportDownloads(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportBodyBytes)

	var doc download.ExportDoc
	if err := c.ShouldBindJSON(&doc); err != nil {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "invalid request body")
		return
	}
	if doc.Version != download.ExportVersion {
		writeError(c, http.StatusBadRequest, dto.CodeValidation, "unsupported list file")
		return
	}
	c.JSON(http.StatusOK, GlobalManager.ImportDownloads(doc))
}
