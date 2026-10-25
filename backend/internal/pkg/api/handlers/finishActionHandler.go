package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetFinishAction reports the running sleep/shutdown countdown. 200 with
// {"pending": null} when none, so the UI can poll without error noise.
func GetFinishAction(c *gin.Context) {
	if p, ok := GlobalManager.PendingFinishAction(); ok {
		c.JSON(http.StatusOK, gin.H{"pending": p})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pending": nil})
}

// CancelFinishAction aborts the countdown; idempotent.
func CancelFinishAction(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"cancelled": GlobalManager.CancelFinishAction()})
}
