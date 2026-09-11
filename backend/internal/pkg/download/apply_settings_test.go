package download

import (
	"testing"
	"time"

	"github.com/tiredbooy/Rum/backend/internal/pkg/config"
)

// newTestManager builds a manager against an isolated config dir so loadFromDisk
// / LoadSettingMetadata never touch the developer's real settings.
func newTestManager(t *testing.T, opt *Options) *JobManager {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := NewJobManager(opt)
	t.Cleanup(m.Shutdown)
	return m
}

// Every live-tunable setting must reach the running engine. Before ApplySettings
// existed, only Connections and the four reliability options were forwarded, so
// changing the download folder, parallel count, retries, silent flag or the
// auto-organize switch did nothing until the app was restarted.
func TestApplySettingsPushesEveryLiveOption(t *testing.T) {
	gov := NewSpeedGovernor(0)
	m := newTestManager(t, &Options{Parallel: 1, Connections: 4, Governor: gov})

	s := config.Setting{
		OutDir:               "/tmp/rum-apply-settings",
		SpeedLimitKB:         256,
		MaxParallel:          5,
		Connections:          12,
		MaxRetries:           7,
		Silent:               true,
		EnableCategories:     true,
		VerifyIntegrity:      true,
		RetryBackoffSec:      9,
		TempDir:              "/tmp/rum-temp",
		KeepPartialOnFailure: false,
	}
	m.ApplySettings(s)

	m.mu.RLock()
	got := *m.opt
	m.mu.RUnlock()

	if got.Out != s.OutDir {
		t.Errorf("Out = %q, want %q", got.Out, s.OutDir)
	}
	if got.SpeedLimit != s.SpeedLimitKB {
		t.Errorf("SpeedLimit = %d, want %d", got.SpeedLimit, s.SpeedLimitKB)
	}
	if got.Parallel != s.MaxParallel {
		t.Errorf("Parallel = %d, want %d", got.Parallel, s.MaxParallel)
	}
	if got.Connections != s.Connections {
		t.Errorf("Connections = %d, want %d", got.Connections, s.Connections)
	}
	if got.MaxRetries != s.MaxRetries {
		t.Errorf("MaxRetries = %d, want %d", got.MaxRetries, s.MaxRetries)
	}
	if !got.Silent {
		t.Error("Silent not applied")
	}
	if !got.Categorize {
		t.Error("Categorize not applied from EnableCategories")
	}
	if got.RetryBackoffSec != s.RetryBackoffSec {
		t.Errorf("RetryBackoffSec = %d, want %d", got.RetryBackoffSec, s.RetryBackoffSec)
	}
	if got.TempDir != s.TempDir {
		t.Errorf("TempDir = %q, want %q", got.TempDir, s.TempDir)
	}
	if got.KeepPartialOnFailure {
		t.Error("KeepPartialOnFailure not applied")
	}
	if gov.LimitKBps() != s.SpeedLimitKB {
		t.Errorf("governor limit = %d, want %d", gov.LimitKBps(), s.SpeedLimitKB)
	}
	// The parallel-download count only means anything if it reaches the
	// concurrency gate, not just the options struct.
	if m.sched.Max() != s.MaxParallel {
		t.Errorf("scheduler max = %d, want %d", m.sched.Max(), s.MaxParallel)
	}
}

// The SSRF guard shapes the HTTP transport the same way the proxy does, so a
// change to it must also rebuild the downloader instead of waiting for a restart.
func TestApplySettingsRebuildsTransportOnSSRFGuardChange(t *testing.T) {
	m := newTestManager(t, &Options{Parallel: 1, Downloader: NewDownloader("", "")})

	base := config.Setting{OutDir: "/tmp", MaxParallel: 1, Connections: 8}
	m.ApplySettings(base)
	m.mu.RLock()
	before := m.opt.Downloader
	m.mu.RUnlock()

	guarded := base
	guarded.BlockPrivateHosts = true
	m.ApplySettings(guarded)
	m.mu.RLock()
	after := m.opt.Downloader
	m.mu.RUnlock()

	if before == after {
		t.Fatal("downloader NOT rebuilt after block_private_hosts changed")
	}
}

// A settings save must not stomp a tighter bandwidth window that is in force
// right now — the schedule controller would only put it back on its next tick.
func TestApplySettingsHonoursActiveBandwidthWindow(t *testing.T) {
	gov := NewSpeedGovernor(0)
	m := newTestManager(t, &Options{Parallel: 1, Governor: gov})

	// An all-day window (start == end) at 100 kB/s, with a looser base limit.
	s := config.Setting{
		OutDir:       "/tmp",
		SpeedLimitKB: 5000,
		MaxParallel:  1,
		Connections:  8,
		BandwidthSchedule: []config.SpeedRule{
			{StartHour: 0, EndHour: 0, LimitKBps: 100},
		},
	}
	m.ApplySettings(s)

	if gov.LimitKBps() != 100 {
		t.Fatalf("governor limit = %d, want the active window's 100", gov.LimitKBps())
	}
}

// The transport is rebuilt only when the proxy or SSRF guard changes, so routine
// saves keep the connection pool and cookie jar.
func TestApplySettingsRebuildsTransportOnlyOnProxyChange(t *testing.T) {
	m := newTestManager(t, &Options{Parallel: 1, Downloader: NewDownloader("", "")})

	base := config.Setting{OutDir: "/tmp", MaxParallel: 1, Connections: 8}

	m.ApplySettings(base)
	m.mu.RLock()
	first := m.opt.Downloader
	m.mu.RUnlock()

	m.ApplySettings(base) // identical save
	m.mu.RLock()
	second := m.opt.Downloader
	m.mu.RUnlock()
	if first != second {
		t.Fatal("downloader rebuilt for an unchanged proxy")
	}

	changed := base
	changed.Proxy = "http://127.0.0.1:3128"
	m.ApplySettings(changed)
	m.mu.RLock()
	third := m.opt.Downloader
	m.mu.RUnlock()
	if third == second {
		t.Fatal("downloader NOT rebuilt after the proxy changed")
	}
}

func TestApplySettingsClampsOutOfRangeValues(t *testing.T) {
	m := newTestManager(t, &Options{Parallel: 1})

	m.ApplySettings(config.Setting{MaxParallel: 0, Connections: 99, MaxRetries: -3})

	m.mu.RLock()
	got := *m.opt
	m.mu.RUnlock()

	if got.Parallel != 1 {
		t.Errorf("Parallel = %d, want clamped to 1", got.Parallel)
	}
	if got.Connections != maxConnections {
		t.Errorf("Connections = %d, want clamped to %d", got.Connections, maxConnections)
	}
	if got.MaxRetries != 0 {
		t.Errorf("MaxRetries = %d, want clamped to 0", got.MaxRetries)
	}
}

func TestEffectiveSpeedLimitFallsBackToGlobal(t *testing.T) {
	s := config.Setting{SpeedLimitKB: 700}
	if got := EffectiveSpeedLimitKBps(s, time.Now()); got != 700 {
		t.Fatalf("EffectiveSpeedLimitKBps = %d, want 700", got)
	}
}
