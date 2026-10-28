package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func onlyHiddenInput(app core.AppID, group core.AppGroup, hidden map[string]bool, runtime core.RuntimeState) SchedulerInput {
	return SchedulerInput{
		Groups:      []core.AppGroup{group},
		Rules:       []core.AppRule{{AppID: app, Mode: core.RuleModePauseInBackground, OnlyWhenHidden: true}},
		Preferences: core.GlobalPreferences{ManagementEnabled: true},
		Runtime:     runtime,
		Frontmost:   core.AppID{BundleID: "com.example.other"},
		HiddenApps:  hidden,
		Now:         time.Unix(10, 0),
	}
}

func hasAction(actions []core.ControlAction, want core.ControlActionType) bool {
	for _, action := range actions {
		if action.Type == want {
			return true
		}
	}
	return false
}

func TestOnlyWhenHiddenPausesWhenHidden(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)

	result := NewScheduler().Evaluate(onlyHiddenInput(app, group, map[string]bool{app.Key(): true}, core.NewRuntimeState()))

	if !hasAction(result.Actions, core.ControlActionPause) {
		t.Fatalf("actions = %#v, want pause", result.Actions)
	}
}

func TestOnlyWhenHiddenSkipsBackgroundAppThatIsNotHidden(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)

	result := NewScheduler().Evaluate(onlyHiddenInput(app, group, map[string]bool{}, core.NewRuntimeState()))

	if hasAction(result.Actions, core.ControlActionPause) {
		t.Fatalf("actions = %#v, want no pause", result.Actions)
	}
}

func TestOnlyWhenHiddenRestoresWhenUnhidden(t *testing.T) {
	app := core.AppID{BundleID: "com.example.app", Name: "Example"}
	group := schedulerGroup(app, 10)
	runtime := core.NewRuntimeState()
	runtime.RecordPause(group.Processes[0], app, core.ControlReasonUserRule, time.Unix(5, 0))

	result := NewScheduler().Evaluate(onlyHiddenInput(app, group, map[string]bool{}, runtime))

	if !hasAction(result.Actions, core.ControlActionResume) {
		t.Fatalf("actions = %#v, want resume", result.Actions)
	}
	if hasAction(result.Actions, core.ControlActionPause) {
		t.Fatalf("actions = %#v, want no re-pause", result.Actions)
	}
}

func TestOnlyWhenHiddenMatchesNameAggregatedGroupsByName(t *testing.T) {
	app := core.AppID{Name: "Example"}
	group := schedulerGroup(app, 10)

	result := NewScheduler().Evaluate(onlyHiddenInput(app, group, map[string]bool{"name:example": true}, core.NewRuntimeState()))

	if !hasAction(result.Actions, core.ControlActionPause) {
		t.Fatalf("actions = %#v, want pause via name fallback", result.Actions)
	}
}
