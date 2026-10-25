package routes

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// gin panics at registration on a conflicting route; export/import sit beside
// /downloads/:id and must not collide with it.
func TestSetupRouterRegistersWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetupRouter(gin.New())
}
