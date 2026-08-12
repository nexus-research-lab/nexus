package protocol

import (
	"strings"
	"testing"
)

func TestGoalReservedExecutionIDRecoversLegacyExplicitReservation(t *testing.T) {
	const commandID = "explicit_goal_legacy_command"
	expected := ExplicitGoalReservedExecutionID(commandID)
	if expected == "" || !strings.HasPrefix(expected, "execution_") {
		t.Fatalf("derived reservation = %q", expected)
	}
	goal := Goal{Metadata: map[string]any{
		GoalMetadataExplicitCommand:  commandID,
		GoalMetadataActivationOrigin: string(GoalActivationOriginUserExplicit),
		GoalMetadataActivationReason: string(GoalActivationReasonPersistenceRequested),
	}}
	if got := GoalReservedExecutionID(goal); got != expected {
		t.Fatalf("legacy reservation = %q, want %q", got, expected)
	}

	goal.Metadata[GoalMetadataExecutionID] = "execution-persisted"
	if got := GoalReservedExecutionID(goal); got != "execution-persisted" {
		t.Fatalf("persisted reservation = %q", got)
	}

	delete(goal.Metadata, GoalMetadataExecutionID)
	goal.Metadata[GoalMetadataActivationOrigin] = string(GoalActivationOriginAdaptiveInitial)
	if got := GoalReservedExecutionID(goal); got != "" {
		t.Fatalf("non-explicit legacy reservation = %q", got)
	}
}

func TestGoalExecutionBindingStateFromGoalKeepsResolverOnlyStatesOutOfMetadata(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  GoalExecutionBindingState
	}{
		{name: "missing is standalone", want: GoalExecutionBindingStateStandalone},
		{name: "reserved", value: "reserved", want: GoalExecutionBindingStateReserved},
		{name: "pending", value: "pending", want: GoalExecutionBindingStatePending},
		{name: "confirmed", value: "confirmed", want: GoalExecutionBindingStateConfirmed},
		{name: "resolver standalone is rejected", value: "standalone", want: GoalExecutionBindingStateConflict},
		{name: "resolver conflict is rejected", value: "conflict", want: GoalExecutionBindingStateConflict},
		{name: "unknown is rejected", value: "unknown", want: GoalExecutionBindingStateConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			goal := Goal{}
			if test.value != nil {
				goal.Metadata = map[string]any{GoalMetadataExecutionBindingState: test.value}
			}
			if got := GoalExecutionBindingStateFromGoal(goal); got != test.want {
				t.Fatalf("binding state = %q, want %q", got, test.want)
			}
		})
	}
}
