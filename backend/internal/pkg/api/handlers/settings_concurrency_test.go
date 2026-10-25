package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// Every settings write is load -> modify -> save of the whole file. Two writes
// in flight at once (two switches flipped quickly, or a PATCH racing the
// schedule PUT) each started from the same snapshot, so the later save erased
// the earlier one: a toggle showed "Saved" and was gone on the next reload.
func TestConcurrentSettingsWritesAreNotLost(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gin.SetMode(gin.TestMode)

	const rounds = 25
	for i := 0; i < rounds; i++ {
		// Start each round from a known baseline.
		if w := patchSettings(t, `{"silent":false,"reduced_motion":false,"confirm_on_exit":false}`); w.Code != http.StatusOK {
			t.Fatalf("baseline status = %d (%s)", w.Code, w.Body.String())
		}

		var wg sync.WaitGroup
		run := func(fn func(*gin.Context), method, body string) {
			defer wg.Done()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			fn(c)
		}
		wg.Add(5)
		go run(UpdateSetting, http.MethodPatch, `{"silent":true}`)
		go run(UpdateSetting, http.MethodPatch, `{"reduced_motion":true}`)
		go run(UpdateSetting, http.MethodPatch, `{"confirm_on_exit":true}`)
		go run(PutSchedule, http.MethodPut, `{"scheduled_start_enabled":false,"rules":[]}`)
		go run(PutCategories, http.MethodPut, `{"enabled":true,"rules":[]}`)
		wg.Wait()

		var got config.Setting
		if err := got.LoadSettingMetadata(); err != nil {
			t.Fatalf("reload: %v", err)
		}
		if !got.Silent || !got.ReducedMotion || !got.ConfirmOnExit ||
			got.ScheduledStartEnabled || !got.EnableCategories {
			t.Fatalf("round %d: a concurrent write was lost: silent=%v reduced=%v confirm=%v schedStart=%v categories=%v",
				i, got.Silent, got.ReducedMotion, got.ConfirmOnExit, got.ScheduledStartEnabled, got.EnableCategories)
		}
	}
}
