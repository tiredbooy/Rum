package routes

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// Registering must not panic on a path conflict (e.g. /downloads/restore vs :id).
func TestSetupRouterRegistersRestoreRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupRouter(r)
	want := map[string]bool{"POST /api/v1/downloads/restore": false, "POST /api/v1/downloads/:id/restore": false}
	for _, ri := range r.Routes() {
		if _, ok := want[ri.Method+" "+ri.Path]; ok {
			want[ri.Method+" "+ri.Path] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("route %s missing", k)
		}
	}
}

// gin panics at registration on a conflicting route; export/import sit beside
// /downloads/:id and must not collide with it.
func TestSetupRouterRegistersWithoutConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetupRouter(gin.New())
}
