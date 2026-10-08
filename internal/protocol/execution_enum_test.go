package protocol

import "testing"

func TestExecutionStatusPartitionsCurrentAndTerminal(t *testing.T) {
	current := []ExecutionStatus{ExecutionStatusActive, ExecutionStatusWaiting, ExecutionStatusPaused}
	terminal := []ExecutionStatus{ExecutionStatusCompleted, ExecutionStatusFailed, ExecutionStatusCancelled, ExecutionStatusSuperseded}
	for _, status := range current {
		if !status.Current() || status.Terminal() {
			t.Fatalf("%s should be current only", status)
		}
	}
	for _, status := range terminal {
		if status.Current() || !status.Terminal() {
			t.Fatalf("%s should be terminal only", status)
		}
	}
	if ExecutionStatus("unknown").Current() || ExecutionStatus("unknown").Terminal() {
		t.Fatal("unknown status must be neither current nor terminal")
	}
}

func TestExecutionEnumValidity(t *testing.T) {
	if !WorkItemKindVerify.Valid() || WorkItemKind("other").Valid() {
		t.Fatal("WorkItemKind.Valid mismatch")
	}
	if !GoalActivationOriginAdaptiveInitial.Valid() || GoalActivationOrigin("").Valid() {
		t.Fatal("GoalActivationOrigin.Valid mismatch")
	}
	if !GoalActivationReasonExternalWait.Valid() || GoalActivationReason("").Valid() {
		t.Fatal("GoalActivationReason.Valid mismatch")
	}
	if GoalActivationReasonPersistenceRequested.PromotionOrigin() != GoalActivationOriginUserExplicit ||
		GoalActivationReasonExternalWait.PromotionOrigin() != GoalActivationOriginAdaptivePromoted {
		t.Fatal("PromotionOrigin mismatch")
	}
}

func TestBindingCloneIsIndependentAndNilSafe(t *testing.T) {
	var nilWork *ExecutionWorkBinding
	var nilReview *ExecutionReviewBinding
	if nilWork.Clone() != nil || nilReview.Clone() != nil {
		t.Fatal("nil binding must clone to nil")
	}
	work := &ExecutionWorkBinding{ExecutionID: "e1"}
	cloned := work.Clone()
	cloned.ExecutionID = "e2"
	if work.ExecutionID != "e1" {
		t.Fatal("Clone must not alias the source")
	}
	if !(GoalUsage{}).IsZero() || (GoalUsage{RuntimeSeconds: 1}).IsZero() {
		t.Fatal("GoalUsage.IsZero mismatch")
	}
}
