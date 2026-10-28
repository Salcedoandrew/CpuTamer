package policy

import (
	"testing"
	"time"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

func TestCPULimitPlannerCreatesDutyCycle(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 50})

	if !plan.Active {
		t.Fatal("expected active plan")
	}
	if plan.StopFor != 5*time.Second {
		t.Fatalf("stop for = %s, want 5s", plan.StopFor)
	}
	if plan.RunFor != 5*time.Second {
		t.Fatalf("run for = %s, want 5s", plan.RunFor)
	}
	if plan.Explanation == "" {
		t.Fatal("expected explanation")
	}
}

func TestCPULimitPlannerCapsBrowserStopWindow(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 10, BrowserLike: true})

	if plan.StopFor != 9*time.Second {
		t.Fatalf("browser stop window = %s, want 9s", plan.StopFor)
	}
}

func TestCPULimitPlannerAllowsPointZeroOnePercent(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: MinCPULimitPercent})

	if !plan.Active {
		t.Fatal("expected active plan")
	}
	if plan.RunFor != time.Millisecond {
		t.Fatalf("run for = %s, want 1ms", plan.RunFor)
	}
	if plan.StopFor != 9999*time.Millisecond {
		t.Fatalf("stop for = %s, want 9999ms", plan.StopFor)
	}
}

// AppPolice-style multi-core target: a multithreaded app observed at 300%
// capped at 200% runs two thirds of each cycle.
func TestCPULimitPlannerSupportsMultiCoreTarget(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 300, TargetCPU: 200})

	if !plan.Active {
		t.Fatal("expected active plan")
	}
	if plan.RunFor != time.Duration(6666666667) {
		t.Fatalf("run for = %s, want 6.666666667s", plan.RunFor)
	}
	if plan.StopFor != time.Duration(3333333333) {
		t.Fatalf("stop for = %s, want 3.333333333s", plan.StopFor)
	}
}

func TestCPULimitPlannerStopsWhenForeground(t *testing.T) {
	planner := DefaultCPULimitPlanner(true)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 50, Foreground: true})

	if plan.Active {
		t.Fatal("foreground app should not be actively limited")
	}
	if plan.Reason != CPULimitReasonForeground {
		t.Fatalf("reason = %q", plan.Reason)
	}
}

func TestCPULimitPlannerHonorsKillSwitch(t *testing.T) {
	planner := DefaultCPULimitPlanner(false)
	plan := planner.Plan(CPULimitInput{ObservedCPU: 100, TargetCPU: 50})

	if plan.Active {
		t.Fatal("disabled planner should not be active")
	}
	if plan.Reason != CPULimitReasonDisabled {
		t.Fatalf("reason = %q", plan.Reason)
	}
}

func TestCPULimitDutyCycleReleasesAfterUnderLimitTicks(t *testing.T) {
	runtime := core.AppControlRuntime{
		Status:          core.AppStatusCPULimited,
		CPULimitTarget:  50,
		CPULimitRunFor:  5 * time.Second,
		CPULimitStopFor: 5 * time.Second,
	}
	underLimit := CPULimitPlan{Active: false, Reason: CPULimitReasonUnderLimit}

	for tick := 1; tick < CPULimitReleaseAfterUnderLimitTicks; tick++ {
		_, _, _, active, ticks := cpuLimitDutyCycle(underLimit, runtime, 50)
		if !active {
			t.Fatalf("tick %d: limit released too early", tick)
		}
		if ticks != tick {
			t.Fatalf("tick %d: under-limit ticks = %d, want %d", tick, ticks, tick)
		}
		runtime.CPULimitUnderLimitTicks = ticks
	}

	_, _, _, active, _ := cpuLimitDutyCycle(underLimit, runtime, 50)
	if active {
		t.Fatal("limit should release after the under-limit tick threshold, not latch on")
	}
}

func TestCPULimitDutyCycleResetsUnderLimitTicksOnOverTarget(t *testing.T) {
	runtime := core.AppControlRuntime{
		Status:                  core.AppStatusCPULimited,
		CPULimitTarget:          50,
		CPULimitRunFor:          5 * time.Second,
		CPULimitStopFor:         5 * time.Second,
		CPULimitUnderLimitTicks: 2,
	}
	overLimit := CPULimitPlan{Active: true, RunFor: 5 * time.Second, StopFor: 5 * time.Second, Explanation: "above target"}

	_, _, _, active, ticks := cpuLimitDutyCycle(overLimit, runtime, 50)
	if !active || ticks != 0 {
		t.Fatalf("active = %v, ticks = %d, want active with 0 ticks", active, ticks)
	}
}
