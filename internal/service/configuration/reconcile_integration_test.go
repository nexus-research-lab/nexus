package configuration_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func TestConfigurationReceiptReviewAndHumanReconcileDoesNotReplay(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	actor := configurationsvc.Actor{
		OwnerUserID:     fixture.main.OwnerUserID,
		AgentID:         fixture.main.AgentID,
		IsMainAgent:     true,
		SessionKey:      "agent:nexus:ws:dm:reconcile-review",
		ContextKind:     configurationsvc.ContextKindAgent,
		ContextID:       fixture.main.AgentID,
		PrincipalRole:   authctx.RoleOwner,
		AuthMethod:      authctx.AuthMethodLocal,
		LocalSingleUser: true,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)

	input := json.RawMessage(`{"agent_sdk_diagnostics_enabled":true}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainPreferences, Operation: "update", Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "reconcile-review-0001",
		Domain:           configurationsvc.DomainPreferences,
		Operation:        "update",
		Input:            input,
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	if _, err = fixture.services.Configuration.ApplyChange(fixture.ownerCtx, actor, request); err != nil {
		t.Fatal(err)
	}
	preferencesBefore, err := fixture.services.Preferences.Get(t.Context(), actor.OwnerUserID)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate the durable recovery sweep after the executor lease expired.
	// The test intentionally changes only the receipt; no second configuration
	// write is allowed during review or reconcile.
	if _, err = fixture.services.DB.ExecContext(
		t.Context(),
		`UPDATE configuration_changes
		 SET status = 'reconcile_required', result_json = ?, revision_after = '',
		     error_message = 'outcome unknown'
		 WHERE owner_user_id = ? AND request_id = ?`,
		`{"applied":"unknown"}`, actor.OwnerUserID, request.RequestID,
	); err != nil {
		t.Fatal(err)
	}

	review, err := fixture.services.Configuration.ReviewChange(t.Context(), actor, request.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if review.Receipt.Status != "reconcile_required" || review.Current.Revision == "" {
		t.Fatalf("review = %+v", review)
	}
	if review.Evidence.DecisionSource != "human_confirmation_required" {
		t.Fatalf("review evidence = %+v", review.Evidence)
	}
	if _, err = fixture.services.Configuration.ReconcileChange(
		t.Context(),
		configurationsvc.Actor{
			OwnerUserID:     fixture.main.OwnerUserID,
			AgentID:         fixture.main.AgentID,
			IsMainAgent:     true,
			SessionKey:      "agent:nexus:ws:dm:stale-review",
			ContextKind:     configurationsvc.ContextKindAgent,
			ContextID:       fixture.main.AgentID,
			PrincipalRole:   authctx.RoleOwner,
			AuthMethod:      authctx.AuthMethodLocal,
			LocalSingleUser: true,
		},
		configurationsvc.ReconcileRequest{
			RequestID: request.RequestID, Decision: "applied",
			ObservedRevision: "stale-revision", Confirmed: true,
		},
	); err == nil || !strings.Contains(err.Error(), "当前配置已变化") {
		t.Fatalf("stale observed revision was accepted: %v", err)
	}

	// A round-scoped model actor cannot submit the human decision. Clearing the
	// lease flag models the local owner configuration CLI after its confirmation
	// boundary has been reached.
	if _, err = fixture.services.Configuration.ReconcileChange(
		t.Context(),
		actor,
		configurationsvc.ReconcileRequest{
			RequestID: request.RequestID, Decision: "applied",
			ObservedRevision: review.Current.Revision, Confirmed: true,
		},
	); err == nil {
		t.Fatal("round-scoped Agent unexpectedly submitted human reconcile")
	}

	humanActor := actor
	humanActor.RoundLeaseRequired = false
	humanActor.LeaseSessionKey = ""
	humanActor.LeaseRoundID = ""
	reconciled, err := fixture.services.Configuration.ReconcileChange(
		t.Context(),
		humanActor,
		configurationsvc.ReconcileRequest{
			RequestID: request.RequestID, Decision: "applied",
			ObservedRevision: review.Current.Revision, Confirmed: true,
			Note: "人工核对当前设置",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Receipt.Status != "reconciled" ||
		reconciled.Evidence.DecisionSource != "human_confirmation" {
		t.Fatalf("reconciled = %+v", reconciled)
	}
	if !strings.Contains(string(reconciled.Receipt.Result), `"decision":"applied"`) ||
		strings.Contains(string(reconciled.Receipt.Result), "人工核对") {
		t.Fatalf("reconcile receipt = %s", reconciled.Receipt.Result)
	}
	preferencesAfter, err := fixture.services.Preferences.Get(t.Context(), actor.OwnerUserID)
	if err != nil {
		t.Fatal(err)
	}
	if preferencesAfter.Version != preferencesBefore.Version {
		t.Fatalf("reconcile replayed a settings write: before=%d after=%d", preferencesBefore.Version, preferencesAfter.Version)
	}
	if _, err = fixture.services.Configuration.ReconcileChange(
		t.Context(), humanActor, configurationsvc.ReconcileRequest{
			RequestID: request.RequestID, Decision: "applied",
			ObservedRevision: review.Current.Revision, Confirmed: true,
		},
	); err == nil || !strings.Contains(err.Error(), "只有 reconcile_required") {
		t.Fatalf("duplicate reconcile was accepted: %v", err)
	}
}
