package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/config"
	"github.com/Digital-Shane/open-tamer/internal/core"
	"github.com/Digital-Shane/open-tamer/internal/observe"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

type fakeRefreshApplications struct {
	frontmost    core.AppID
	frontmostErr error
}

func (fake *fakeRefreshApplications) AppProcessHints() ([]core.AppProcessHint, error) {
	return nil, nil
}

func (fake *fakeRefreshApplications) FrontmostAppID() (core.AppID, error) {
	return fake.frontmost, fake.frontmostErr
}

type fakeRefreshProcesses struct {
	processes []core.ProcessRef
}

func (fake *fakeRefreshProcesses) Processes() ([]core.ProcessRef, error) {
	return fake.processes, nil
}

type fakeRefreshMetrics struct{}

func (fake *fakeRefreshMetrics) SampleProcesses([]core.ProcessRef) ([]core.ProcessMetrics, error) {
	return nil, nil
}

func (fake *fakeRefreshMetrics) SampleSystemCPU() (core.SystemCPUSample, error) {
	return core.SystemCPUSample{SampledAt: time.Now()}, nil
}

type fakeRefreshAudio struct{}

func (fake *fakeRefreshAudio) AudioActive() (bool, error) { return false, nil }

type fakeRefreshSystem struct{}

func (fake *fakeRefreshSystem) Snapshot() (core.SystemPolicyState, error) {
	return core.SystemPolicyState{}, nil
}

type fakeRefreshNotifier struct{}

func (fake *fakeRefreshNotifier) NotifyHighCPU(HighCPUNotice) error { return nil }

// A failed frontmost lookup must not pause the app in use: the refresh
// must freeze controls for that tick instead of treating every app as
// background. The same controller pauses the app once the lookup
// succeeds, proving the rule would otherwise have applied.
func TestRefreshSkipsControlsWhenFrontmostLookupFails(t *testing.T) {
	app := core.AppID{Name: "Example"}
	cfg := config.DefaultConfig()
	cfg.Rules = []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground}}

	applications := &fakeRefreshApplications{frontmostErr: errors.New("frontmost unavailable")}
	signals := &fakeSignals{}
	controller := &Controller{
		cfg:               cfg,
		store:             config.NewStore(t.TempDir()),
		runtime:           core.NewRuntimeState(),
		stats:             core.StatsDocument{},
		lastStatuses:      make(map[string]core.AppStatus),
		applications:      applications,
		processes:         &fakeRefreshProcesses{processes: []core.ProcessRef{{ID: core.ProcessID{PID: 42}, UID: 501, Name: "Example", ExecutablePath: "/Applications/Example.app/Contents/MacOS/Example"}}},
		metrics:           &fakeRefreshMetrics{},
		audio:             &fakeRefreshAudio{},
		system:            &fakeRefreshSystem{},
		notifier:          &fakeRefreshNotifier{},
		scheduler:         apppolicy.NewScheduler(),
		cpu:               observe.NewCPUUsageCalculator(),
		systemCPU:         &observe.SystemCPUCalculator{},
		highCPU:           NewHighCPUDetector(),
		groupCtl:          &AppGroupController{Signals: signals},
		lastRuntimeSaveAt: time.Now(),
	}

	controller.Refresh()
	if len(signals.stopped) != 0 {
		t.Fatalf("stopped = %v, want no pause while frontmost is unknown", signals.stopped)
	}
	if len(controller.lastGroups) != 1 {
		t.Fatalf("observed groups = %d, want 1 (observations still update)", len(controller.lastGroups))
	}

	applications.frontmostErr = nil
	applications.frontmost = core.AppID{Name: "Other"}
	controller.Refresh()
	if len(signals.stopped) != 1 {
		t.Fatalf("stopped = %v, want one pause once frontmost is known", signals.stopped)
	}
}
