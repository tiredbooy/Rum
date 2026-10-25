package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func newTestService(t *testing.T, current string, srv *httptest.Server, now *time.Time) *Service {
	t.Helper()
	return &Service{
		Current:   current,
		StatePath: filepath.Join(t.TempDir(), "update_state.json"),
		Now:       func() time.Time { return *now },
		NewChecker: func(Options) *Checker {
			return &Checker{Client: srv.Client(), APIURL: srv.URL}
		},
	}
}

func TestAutoCheckThrottledOncePerDayAndPersisted(t *testing.T) {
	var hits int32
	srv := releaseServer(t, &hits)
	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	s := newTestService(t, "1.0.0", srv, &now)
	opts := Options{Enabled: true}

	st := s.CheckAuto(context.Background(), opts)
	if !st.Available || st.Latest != "2.0.0" || hits != 1 {
		t.Fatalf("first: %+v hits=%d", st, hits)
	}
	now = now.Add(23 * time.Hour)
	st = s.CheckAuto(context.Background(), opts)
	if hits != 1 || !st.Available {
		t.Fatalf("throttled call hit network or lost result: %+v hits=%d", st, hits)
	}

	// A fresh Service on the same file (app restart) is still throttled.
	s2 := &Service{Current: "1.0.0", StatePath: s.StatePath, Now: s.Now, NewChecker: s.NewChecker}
	if st := s2.CheckAuto(context.Background(), opts); hits != 1 || !st.Available {
		t.Fatalf("restart re-checked: %+v hits=%d", st, hits)
	}

	now = now.Add(2 * time.Hour) // 25h since first
	s2.CheckAuto(context.Background(), opts)
	if hits != 2 {
		t.Fatalf("hits = %d, want 2 after a day", hits)
	}
}

func TestAutoCheckDisabledNeverHitsNetwork(t *testing.T) {
	var hits int32
	srv := releaseServer(t, &hits)
	now := time.Now()
	s := newTestService(t, "1.0.0", srv, &now)
	st := s.CheckAuto(context.Background(), Options{Enabled: false})
	if hits != 0 || st.Available {
		t.Fatalf("disabled check ran: %+v hits=%d", st, hits)
	}
}

func TestManualCheckIgnoresThrottle(t *testing.T) {
	var hits int32
	srv := releaseServer(t, &hits)
	now := time.Now()
	s := newTestService(t, "1.0.0", srv, &now)
	s.CheckAuto(context.Background(), Options{Enabled: true})
	s.CheckNow(context.Background(), Options{Enabled: false}) // works even if auto is off
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
}

func TestUpToDateAndFailureStates(t *testing.T) {
	srv := releaseServer(t, nil)
	now := time.Now()
	s := newTestService(t, "2.0.0", srv, &now)
	if st := s.CheckNow(context.Background(), Options{}); st.Available || st.Latest != "2.0.0" || st.Error != "" {
		t.Errorf("up to date: %+v", st)
	}

	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	s.NewChecker = func(Options) *Checker { return &Checker{Client: http.DefaultClient, APIURL: dead.URL} }
	st := s.CheckNow(context.Background(), Options{})
	if st.Error != ErrCodeUpdateCheck || st.Available {
		t.Errorf("failure: %+v", st)
	}
	// A failed check must not erase the last known good result.
	if st.Latest != "2.0.0" {
		t.Errorf("lost last known release: %+v", st)
	}
}

func TestDismissHidesUntilNewerRelease(t *testing.T) {
	srv := releaseServer(t, nil)
	now := time.Now()
	s := newTestService(t, "1.0.0", srv, &now)
	s.CheckNow(context.Background(), Options{})
	if err := s.Dismiss("v2.0.0"); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if !st.Available || !st.Dismissed {
		t.Fatalf("after dismiss: %+v", st)
	}
	// Persisted across restart.
	s2 := &Service{Current: "1.0.0", StatePath: s.StatePath, Now: s.Now}
	if !s2.Status().Dismissed {
		t.Error("dismissal not persisted")
	}
	if err := s.Dismiss("bogus tag"); err == nil {
		t.Error("invalid tag must be rejected")
	}
}
