package updater

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/download"
	filesystem "github.com/tiredbooy/Rum/backend/internal/pkg/file-system"
)

// CheckInterval is the minimum gap between automatic checks.
const CheckInterval = 24 * time.Hour

// Error codes surfaced in Status.Error (defined with the other codes).
const (
	ErrCodeUpdateCheck  = download.ErrCodeUpdateCheck
	ErrCodeUpdateVerify = download.ErrCodeUpdateVerify
)

// Options are the per-call settings the service needs.
type Options struct {
	Enabled bool     // "Check for updates"; gates automatic checks only
	Proxy   string   // Rum's proxy setting
	Mirrors []string // user mirror list; nil = DefaultMirrors
}

// Status is what the UI shows. Latest/ReleaseURL keep the last good result even
// when the newest check failed.
type Status struct {
	Current    string    `json:"current"`
	Latest     string    `json:"latest,omitempty"`
	Tag        string    `json:"tag,omitempty"`
	Available  bool      `json:"available"`
	Dismissed  bool      `json:"dismissed"`
	ReleaseURL string    `json:"release_url,omitempty"`
	Asset      string    `json:"asset,omitempty"`
	CheckedAt  time.Time `json:"checked_at,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type state struct {
	CheckedAt time.Time `json:"checked_at"`
	Release   *Release  `json:"release,omitempty"`
	Dismissed string    `json:"dismissed,omitempty"`
}

// Service runs checks and persists the throttle/dismissal state.
type Service struct {
	Current    string
	StatePath  string // defaults to <data dir>/update_state.json
	Now        func() time.Time
	NewChecker func(Options) *Checker

	mu sync.Mutex
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) path() string {
	if s.StatePath != "" {
		return s.StatePath
	}
	return filesystem.CreateMetadataFile("update_state.json")
}

func (s *Service) load() state {
	var st state
	if b, err := os.ReadFile(s.path()); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

func (s *Service) save(st state) {
	if b, err := json.MarshalIndent(st, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(s.path()), 0o755)
		_ = filesystem.AtomicWriteFile(s.path(), b, 0o644)
	}
}

func (s *Service) checker(o Options) *Checker {
	if s.NewChecker != nil {
		return s.NewChecker(o)
	}
	m := o.Mirrors
	if m == nil {
		m = DefaultMirrors
	}
	return &Checker{Client: NewClient(o.Proxy), Mirrors: m}
}

func (s *Service) status(st state, errCode string) Status {
	out := Status{Current: s.Current, CheckedAt: st.CheckedAt, Error: errCode}
	if st.Release == nil {
		return out
	}
	out.Latest, out.Tag, out.ReleaseURL = st.Release.Version, st.Release.Tag, st.Release.PageURL
	out.Available = IsNewer(st.Release.Version, s.Current)
	out.Dismissed = out.Available && st.Dismissed == st.Release.Tag
	return out
}

// Status reports the last known result without touching the network.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status(s.load(), "")
}

// CheckAuto is the startup check: skipped when disabled or when a check ran in
// the last CheckInterval (persisted, so restarts do not re-check). Failures are
// recorded too, so an unreachable network is retried tomorrow, not every launch.
func (s *Service) CheckAuto(ctx context.Context, o Options) Status {
	s.mu.Lock()
	st := s.load()
	due := o.Enabled && (st.CheckedAt.IsZero() || s.now().Sub(st.CheckedAt) >= CheckInterval || st.CheckedAt.After(s.now()))
	if !due {
		defer s.mu.Unlock()
		return s.status(st, "")
	}
	s.mu.Unlock()
	return s.CheckNow(ctx, o)
}

// CheckNow checks immediately (the "Check now" button), ignoring the throttle.
func (s *Service) CheckNow(ctx context.Context, o Options) Status {
	rel, err := s.checker(o).Latest(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.load()
	st.CheckedAt = s.now()
	code := ""
	if err != nil {
		code = ErrCodeUpdateCheck
	} else {
		st.Release = &rel
	}
	s.save(st)
	return s.status(st, code)
}

// Dismiss hides the notice for this release ("Later"). A newer release shows again.
func (s *Service) Dismiss(tag string) error {
	if !safeTag.MatchString(tag) {
		return errors.New("invalid release tag")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.load()
	st.Dismissed = tag
	s.save(st)
	return nil
}

// Download saves and verifies the installer for this system into dir.
func (s *Service) Download(ctx context.Context, o Options, goos, goarch, dir string) (string, error) {
	s.mu.Lock()
	st := s.load()
	s.mu.Unlock()
	if st.Release == nil || !IsNewer(st.Release.Version, s.Current) {
		return "", ErrNoAsset
	}
	return s.checker(o).DownloadVerified(ctx, *st.Release, goos, goarch, dir)
}
