package drift

import "testing"

func TestErrorOutputPrefersInitOutputWhenInitFailed(t *testing.T) {
	r := DriftProjectResult{
		Succeeded:   false,
		FailedPhase: PhaseInit,
		InitOutput:  "Error: Failed to install provider",
		PlanOutput:  "",
	}

	if got := r.ErrorOutput(); got != "Error: Failed to install provider" {
		t.Errorf("ErrorOutput() = %q, want the init output", got)
	}
}

func TestErrorOutputUsesPlanOutputWhenPlanFailed(t *testing.T) {
	r := DriftProjectResult{
		Succeeded:   false,
		FailedPhase: PhasePlan,
		InitOutput:  "",
		PlanOutput:  "Planning failed.",
	}

	if got := r.ErrorOutput(); got != "Planning failed." {
		t.Errorf("ErrorOutput() = %q, want the plan output", got)
	}
}

func TestErrorOutputFallsBackWhenPhaseUnset(t *testing.T) {
	tests := []struct {
		name       string
		initOutput string
		planOutput string
		want       string
	}{
		{"init output only", "init boom", "", "init boom"},
		{"plan output only", "", "plan boom", "plan boom"},
		{"neither", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := DriftProjectResult{Succeeded: false, InitOutput: tt.initOutput, PlanOutput: tt.planOutput}
			if got := r.ErrorOutput(); got != tt.want {
				t.Errorf("ErrorOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorOutputFallsBackWhenInitPhaseHasNoInitOutput(t *testing.T) {
	r := DriftProjectResult{Succeeded: false, FailedPhase: PhaseInit, InitOutput: "", PlanOutput: "leftover"}

	if got := r.ErrorOutput(); got != "leftover" {
		t.Errorf("ErrorOutput() = %q, want the plan output as fallback", got)
	}
}
