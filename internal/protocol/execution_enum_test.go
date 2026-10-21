package protocol

import "testing"

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
