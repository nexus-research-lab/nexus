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
