// integration_test.go 覆盖配置服务主链路、对账、重启修订与会话集成测试。
package configuration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	"github.com/nexus-research-lab/nexus/internal/storage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestConfigurationControlPlaneAppliesAndVerifiesPreferenceChange(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	services, mainAgent := fixture.services, fixture.main
	db := services.DB
	actor := configurationsvc.Actor{
		OwnerUserID: mainAgent.OwnerUserID, AgentID: mainAgent.AgentID, IsMainAgent: true,
		SessionKey: "agent:nexus:ws:dm:main", ContextKind: configurationsvc.ContextKindAgent,
		ContextID:     mainAgent.AgentID,
		PrincipalRole: authctx.RoleOwner, AuthMethod: authctx.AuthMethodLocal,
		LocalSingleUser: true,
	}
	bindConfigurationTestRound(t, services, &actor)
	before, err := services.Configuration.Inspect(t.Context(), actor, []string{
		configurationsvc.DomainPreferences,
		configurationsvc.DomainProviders,
		configurationsvc.DomainAgents,
		configurationsvc.DomainChannels,
		configurationsvc.DomainConnectors,
		configurationsvc.DomainSkills,
		configurationsvc.DomainHost,
	}, true)
	if err != nil {
		t.Fatalf("inspect full mutable configuration: %v", err)
	}
	preferences := before.Domains[configurationsvc.DomainPreferences]
	input := json.RawMessage(`{"agent_sdk_diagnostics_enabled":true}`)
	plan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainPreferences, Operation: "update", Input: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentRevision != preferences.Revision {
		t.Fatalf("plan revision = %q, inspect revision = %q", plan.CurrentRevision, preferences.Revision)
	}
	if plan.StateVersion <= 0 || plan.StateVersion != preferences.StateVersion {
		t.Fatalf("Preferences plan state_version = %d, inspect = %d", plan.StateVersion, preferences.StateVersion)
	}
	applied, err := services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-pref-0001", Domain: configurationsvc.DomainPreferences,
		Operation: "update", Input: input, ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.RevisionAfter == applied.RevisionBefore {
		t.Fatalf("apply result = %+v", applied)
	}
	stored, err := services.Preferences.Get(t.Context(), actor.OwnerUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.AgentSDKDiagnosticsEnabled {
		t.Fatal("preference change did not reach source of truth")
	}
	if stored.Version != plan.StateVersion+1 {
		t.Fatalf("Preferences CAS version = %d, want %d", stored.Version, plan.StateVersion+1)
	}
	replayed, err := services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-pref-0001", Domain: configurationsvc.DomainPreferences,
		Operation: "update", Input: input, ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.IdempotentReplay {
		t.Fatalf("repeated request was not replayed: %+v", replayed)
	}
	changes, err := services.Configuration.ListChanges(t.Context(), actor, configurationsvc.DomainPreferences, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Status != "success" {
		t.Fatalf("audit changes = %+v", changes)
	}

	const providerSecret = "provider-secret-must-not-leak"
	providerInput := json.RawMessage(`{
		"provider":"dialog-provider",
		"preset_key":"custom",
		"api_format":"responses",
		"display_name":"Dialog Provider",
		"auth_token":{"$secret":"provider.auth_token"},
		"base_url":"https://provider.example.com/v1",
		"models_path":"/models"
	}`)
	providerPlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "create", Input: providerInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	providerRequest := configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0001", Domain: configurationsvc.DomainProviders,
		Operation: "create", Input: providerInput, ExpectedRevision: providerPlan.CurrentRevision,
		PlanDigest: providerPlan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		services,
		t.Context(),
		actor,
		providerRequest,
		providerPlan,
		map[string]string{"provider.auth_token": providerSecret},
	)
	providerApplied, err := services.Configuration.ApplyChange(t.Context(), actor, providerRequest)
	if err != nil {
		t.Fatal(err)
	}
	appliedJSON, err := json.Marshal(providerApplied)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(appliedJSON), providerSecret) {
		t.Fatalf("apply result leaked Provider secret: %s", appliedJSON)
	}
	createdProvider, err := services.Provider.Get(t.Context(), "dialog-provider")
	if err != nil {
		t.Fatal(err)
	}
	if !createdProvider.Enabled {
		t.Fatal("conversational Provider create must default enabled=true")
	}
	if createdProvider.ConfigurationVersion != 1 {
		t.Fatalf("created Provider configuration_version = %d, want 1", createdProvider.ConfigurationVersion)
	}

	updateInput := json.RawMessage(`{"display_name":"Renamed Provider"}`)
	updatePlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "update",
		Target: "dialog-provider", Input: updateInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatePlan.CurrentRevision == "" ||
		updatePlan.Scope.Kind != configurationsvc.ScopeKindOwner ||
		updatePlan.StateVersion != createdProvider.ConfigurationVersion {
		t.Fatalf("provider target plan is incomplete: %+v", updatePlan)
	}
	updateRequest := configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0002", Domain: configurationsvc.DomainProviders,
		Operation: "update", Target: "dialog-provider", Input: updateInput,
		ExpectedRevision: updatePlan.CurrentRevision, PlanDigest: updatePlan.PlanDigest,
	}
	approveConfigurationTestChange(t, services, t.Context(), actor, updateRequest, updatePlan)
	updatedApply, err := services.Configuration.ApplyChange(t.Context(), actor, updateRequest)
	if err != nil {
		t.Fatal(err)
	}
	updatedProvider, err := services.Provider.Get(t.Context(), "dialog-provider")
	if err != nil {
		t.Fatal(err)
	}
	if !updatedProvider.Enabled || updatedProvider.DisplayName != "Renamed Provider" {
		t.Fatalf("Provider merge patch reset existing configuration: %+v", updatedProvider)
	}
	if updatedProvider.ConfigurationVersion != updatePlan.StateVersion+1 ||
		!hasConfigurationCheck(updatedApply.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Provider update was not CAS-verified: provider=%+v apply=%+v", updatedProvider, updatedApply)
	}
	var storedToken string
	if err = db.QueryRow(`SELECT auth_token FROM provider WHERE provider = 'dialog-provider'`).Scan(&storedToken); err != nil {
		t.Fatal(err)
	}
	if storedToken != providerSecret {
		t.Fatal("Provider merge patch did not preserve omitted auth_token")
	}
	fallbackProvider, err := services.Provider.Create(t.Context(), providersvc.CreateInput{
		Provider: "integration-fallback", PresetKey: "custom",
		APIFormat:   providersvc.APIFormatAnthropicMessages,
		DisplayName: "Integration Fallback", AuthToken: "fallback-token",
		BaseURL: "https://fallback.example.com", ModelsPath: "/models", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = services.Provider.UpdateModel(
		t.Context(),
		fallbackProvider.Provider,
		"fallback-model",
		providersvc.UpdateModelInput{Enabled: true, IsDefault: true},
	); err != nil {
		t.Fatal(err)
	}
	deletePlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "delete",
		Target: "dialog-provider", Input: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if deletePlan.StateVersion != updatedProvider.ConfigurationVersion {
		t.Fatalf("Provider delete plan version = %d, want %d",
			deletePlan.StateVersion, updatedProvider.ConfigurationVersion)
	}
	deleteRequest := configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0003", Domain: configurationsvc.DomainProviders,
		Operation: "delete", Target: "dialog-provider", Input: json.RawMessage(`{}`),
		ExpectedRevision: deletePlan.CurrentRevision, PlanDigest: deletePlan.PlanDigest,
	}
	approveConfigurationTestChange(t, services, t.Context(), actor, deleteRequest, deletePlan)
	deletedProvider, err := services.Configuration.ApplyChange(t.Context(), actor, deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(deletedProvider.Checks, "configuration_target_deleted") {
		t.Fatalf("Provider deletion was not verified against source of truth: %+v", deletedProvider)
	}
	if _, err = services.Provider.Get(t.Context(), "dialog-provider"); err == nil {
		t.Fatal("Provider still exists after verified delete")
	}
	allChanges, err := services.Configuration.ListChanges(t.Context(), actor, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	auditJSON, err := json.Marshal(allChanges)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(auditJSON), providerSecret) {
		t.Fatalf("configuration audit leaked Provider secret: %s", auditJSON)
	}
	var deleteAudit *configurationsvc.AuditRecord
	for index := range allChanges {
		if allChanges[index].RequestID == deleteRequest.RequestID {
			deleteAudit = &allChanges[index]
			break
		}
	}
	if deleteAudit == nil ||
		deleteAudit.HumanApprovalRequestID != "perm-"+deleteRequest.RequestID ||
		deleteAudit.HumanPrincipalUserID != actor.OwnerUserID ||
		deleteAudit.HumanPrincipalRole != authctx.RoleOwner ||
		deleteAudit.HumanAuthMethod != authctx.AuthMethodLocal ||
		deleteAudit.HumanApprovedAt == nil {
		t.Fatalf("destructive change audit lost human approval evidence: %+v", deleteAudit)
	}
}

func hasConfigurationCheck(checks []configurationsvc.Check, code string) bool {
	for _, check := range checks {
		if check.Code == code && check.Verified {
			return true
		}
	}
	return false
}

type configurationTestPrincipalVerifier struct{}

func (configurationTestPrincipalVerifier) VerifyInteractiveHuman(
	_ context.Context,
	principal *authctx.Principal,
) (*authctx.Principal, error) {
	if principal == nil {
		return nil, fmt.Errorf("missing test principal")
	}
	copyPrincipal := *principal
	return &copyPrincipal, nil
}

func (configurationTestPrincipalVerifier) AcquireBoundInteractiveHumanLease(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*authctx.Principal, func(), error) {
	principal := authctx.PrincipalFromContext(ctx)
	if principal == nil {
		principal = &authctx.Principal{
			UserID: userID, Role: authctx.RoleOwner, AuthMethod: authMethod,
		}
		if strings.TrimSpace(sessionID) != "" {
			value := strings.TrimSpace(sessionID)
			principal.SessionID = &value
		}
	}
	actualSessionID := ""
	if principal.SessionID != nil {
		actualSessionID = strings.TrimSpace(*principal.SessionID)
	}
	if principal.UserID != userID ||
		principal.AuthMethod != authMethod ||
		actualSessionID != strings.TrimSpace(sessionID) {
		return nil, nil, fmt.Errorf("test principal lease binding mismatch")
	}
	copyPrincipal := *principal
	return &copyPrincipal, func() {}, nil
}

func (configurationTestPrincipalVerifier) ResolveActivePrincipalRole(
	_ context.Context,
	_ string,
) (string, error) {
	return authctx.RoleOwner, nil
}

func enableConfigurationTestPrincipalVerification(services *app.AppServices) {
	verifier := configurationTestPrincipalVerifier{}
	services.Configuration.SetPrincipalVerifiers(verifier, verifier)
}

func bindConfigurationTestRound(
	t *testing.T,
	services *app.AppServices,
	actor *configurationsvc.Actor,
) {
	t.Helper()
	if actor == nil {
		t.Fatal("nil configuration actor")
	}
	if strings.TrimSpace(actor.SessionKey) == "" {
		t.Fatal("configuration test actor requires session key")
	}
	switch actor.ContextKind {
	case configurationsvc.ContextKindRoom:
		roomID := strings.TrimSpace(actor.RoomID)
		if roomID == "" {
			roomID = strings.TrimSpace(actor.ContextID)
		}
		actor.SourceContext = configurationsvc.ContextKindRoom + ":" + roomID
	case configurationsvc.ContextKindAgent:
		actor.SourceContext = configurationsvc.ContextKindAgent + ":" + strings.TrimSpace(actor.AgentID)
	}
	actor.RoundID = "round-" + strings.NewReplacer(":", "-", "/", "-").Replace(actor.SessionKey)
	if strings.TrimSpace(actor.LeaseSessionKey) == "" {
		actor.LeaseSessionKey = actor.SessionKey
	}
	if actor.ContextKind == configurationsvc.ContextKindRoom {
		actor.LeaseRoundID = "agent-" + actor.RoundID
	} else {
		actor.LeaseRoundID = actor.RoundID
	}
	actor.RoundLeaseRequired = true
	if err := services.Runtime.StartRound(
		t.Context(),
		actor.LeaseSessionKey,
		actor.LeaseRoundID,
		nil,
	); err != nil {
		t.Fatalf("start configuration test round %s: %v", actor.LeaseRoundID, err)
	}
	t.Cleanup(func() {
		services.Runtime.MarkRoundFinished(actor.LeaseSessionKey, actor.LeaseRoundID)
	})
}

func approveConfigurationTestChange(
	t *testing.T,
	services *app.AppServices,
	ctx context.Context,
	actor configurationsvc.Actor,
	request configurationsvc.ChangeRequest,
	plan *configurationsvc.ChangePlan,
) {
	approveConfigurationTestChangeWithSecrets(
		t,
		services,
		ctx,
		actor,
		request,
		plan,
		nil,
	)
}

func approveConfigurationTestChangeWithSecrets(
	t *testing.T,
	services *app.AppServices,
	ctx context.Context,
	actor configurationsvc.Actor,
	request configurationsvc.ChangeRequest,
	plan *configurationsvc.ChangePlan,
	secrets map[string]string,
) {
	t.Helper()
	if plan == nil || !plan.RequiresConfirmation {
		return
	}
	if authctx.PrincipalFromContext(ctx) == nil {
		role := strings.TrimSpace(actor.PrincipalRole)
		if role == "" {
			role = authctx.RoleOwner
		}
		authMethod := strings.TrimSpace(actor.AuthMethod)
		if authMethod == "" {
			authMethod = authctx.AuthMethodLocal
		}
		ctx = authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID: actor.OwnerUserID, Username: actor.AgentID,
			Role: role, AuthMethod: authMethod,
		})
	}
	var input any = map[string]any{}
	if len(request.Input) > 0 {
		if err := json.Unmarshal(request.Input, &input); err != nil {
			t.Fatalf("decode configuration approval input: %v", err)
		}
	}
	toolInput := map[string]any{
		"request_id":        request.RequestID,
		"domain":            request.Domain,
		"operation":         request.Operation,
		"target":            request.Target,
		"input":             input,
		"expected_revision": request.ExpectedRevision,
		"plan_digest":       request.PlanDigest,
	}
	route := permissionctx.RouteContext{
		DispatchSessionKey: actor.SessionKey,
		AgentID:            actor.AgentID,
		RoundID:            actor.RoundID,
		AgentRoundID:       actor.LeaseRoundID,
		RoomID:             actor.RoomID,
		ConversationID:     actor.ConversationID,
	}
	if err := services.Configuration.RecordHumanToolApproval(
		ctx,
		permissionctx.HumanToolApproval{
			PermissionRequestID:      "perm-" + request.RequestID,
			ToolName:                 "mcp__nexus_config__apply_nexus_configuration_change",
			ToolInput:                toolInput,
			ConfigurationSecrets:     secrets,
			ConfigurationSecretSlots: plan.SecretSlots,
			RuntimeSessionKey:        actor.LeaseSessionKey,
			DispatchSessionKey:       actor.SessionKey,
			Route:                    route,
			ExpiresAt:                time.Now().Add(time.Minute),
		},
	); err != nil {
		t.Fatalf("record configuration human approval: %v", err)
	}
}

type fixedConfigurationRoleResolver struct {
	role string
}

func (r fixedConfigurationRoleResolver) ResolveActivePrincipalRole(context.Context, string) (string, error) {
	return r.role, nil
}

func TestMemberMainAuditListingExcludesHostConfiguration(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	db, err := storage.OpenDB(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, record := range []struct {
		requestID string
		domain    string
	}{
		{requestID: "audit-host-boundary", domain: configurationsvc.DomainHost},
		{requestID: "audit-private-boundary", domain: configurationsvc.DomainPreferences},
	} {
		if _, err = db.ExecContext(
			t.Context(),
			`INSERT INTO configuration_changes (
				request_id, owner_user_id, actor_agent_id, domain, operation,
				scope_kind, scope_id, authority, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.requestID,
			fixture.main.OwnerUserID,
			fixture.main.AgentID,
			record.domain,
			"inspect",
			configurationsvc.ScopeKindOwner,
			fixture.main.OwnerUserID,
			configurationsvc.AuthorityOwnerMain,
			"success",
		); err != nil {
			t.Fatal(err)
		}
	}

	fixture.services.Configuration.SetPrincipalVerifiers(
		configurationTestPrincipalVerifier{},
		fixedConfigurationRoleResolver{role: authctx.RoleMember},
	)
	memberCtx := authctx.WithPrincipal(t.Context(), &authctx.Principal{
		UserID: fixture.main.OwnerUserID, Role: authctx.RoleMember,
		AuthMethod: authctx.AuthMethodPassword,
	})
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:audit-boundary",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	records, err := fixture.services.Configuration.ListChanges(memberCtx, actor, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Domain == configurationsvc.DomainHost {
			t.Fatalf("member main Agent received host audit record: %+v", record)
		}
	}
	if _, err = fixture.services.Configuration.ListChanges(
		memberCtx,
		actor,
		configurationsvc.DomainHost,
		20,
	); err == nil {
		t.Fatal("member main Agent explicitly read host audit history")
	}
}

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

func TestConfigurationRevisionSurvivesDatabaseReopen(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID, AgentID: fixture.main.AgentID,
		IsMainAgent: true, ContextKind: configurationsvc.ContextKindAgent,
		ContextID: fixture.main.AgentID, PrincipalRole: authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal, LocalSingleUser: true,
	}
	change := configurationsvc.ChangeRequest{
		RequestID: "revision-restart-0001", Domain: configurationsvc.DomainPreferences,
		Operation: "update", Input: json.RawMessage(`{"agent_sdk_diagnostics_enabled":true}`),
	}
	plan, err := fixture.services.Configuration.PlanChange(t.Context(), actor, change)
	if err != nil {
		t.Fatal(err)
	}
	change.ExpectedRevision, change.PlanDigest = plan.CurrentRevision, plan.PlanDigest
	result, err := fixture.services.Configuration.ApplyChange(t.Context(), actor, change)
	if err != nil {
		t.Fatal(err)
	}
	// Keep a durable after-revision while simulating an executor whose final
	// outcome requires human review. Restart must not change that comparison.
	if _, err = fixture.services.DB.ExecContext(t.Context(),
		`UPDATE configuration_changes SET status = 'reconcile_required', result_json = '{"applied":"unknown"}'
		 WHERE owner_user_id = ? AND request_id = ?`, actor.OwnerUserID, change.RequestID,
	); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.services.Configuration.ReviewChange(t.Context(), actor, change.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Evidence.RevisionRelation != "matches_recorded_after" {
		t.Fatalf("pre-restart revision relation = %s", before.Evidence.RevisionRelation)
	}
	unapplied := configurationsvc.ChangeRequest{
		RequestID: "revision-old-plan-0001", Domain: change.Domain, Operation: change.Operation,
		Input: json.RawMessage(`{"agent_sdk_diagnostics_enabled":false}`),
	}
	oldPlan, err := fixture.services.Configuration.PlanChange(t.Context(), actor, unapplied)
	if err != nil {
		t.Fatal(err)
	}
	if err = fixture.services.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = fixture.services.DB.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	restarted := app.NewAppServicesWithDB(fixture.config, db, nil)
	enableConfigurationTestPrincipalVerification(restarted)
	t.Cleanup(func() { _ = restarted.Close(t.Context()) })
	after, err := restarted.Configuration.ReviewChange(t.Context(), actor, change.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Current.Revision != result.RevisionAfter || after.Evidence.RevisionRelation != "matches_recorded_after" {
		t.Fatalf("unchanged configuration lost its durable revision after database reopen: relation=%s", after.Evidence.RevisionRelation)
	}
	freshPlan, err := restarted.Configuration.PlanChange(t.Context(), actor, unapplied)
	if err != nil {
		t.Fatal(err)
	}
	if freshPlan.CurrentRevision != oldPlan.CurrentRevision || freshPlan.PlanDigest == oldPlan.PlanDigest {
		t.Fatal("revision must survive restart while the approval digest must expire")
	}
	unapplied.ExpectedRevision, unapplied.PlanDigest = oldPlan.CurrentRevision, oldPlan.PlanDigest
	if _, err = restarted.Configuration.ApplyChange(t.Context(), actor, unapplied); err == nil || !strings.Contains(err.Error(), "plan_digest") {
		t.Fatalf("pre-restart plan was not rejected: %v", err)
	}
	reconciled, err := restarted.Configuration.ReconcileChange(t.Context(), actor, configurationsvc.ReconcileRequest{
		RequestID: change.RequestID, Decision: "applied", Confirmed: true,
		ObservedRevision: before.Current.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Receipt.Status != "reconciled" || reconciled.Current.StateVersion != before.Current.StateVersion {
		t.Fatalf("reconcile changed the configuration: status=%s version=%d", reconciled.Receipt.Status, reconciled.Current.StateVersion)
	}
	if _, err = restarted.Configuration.ReviewChange(t.Context(), actor, unapplied.RequestID); err == nil {
		t.Fatal("rejected pre-restart plan created an execution receipt")
	}
}

func TestSessionConfigurationConversationBoundaryCASAndDeletion(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Session Configuration Worker")
	targetSessionKey := "agent:" + worker.AgentID + ":ws:dm:session-config-target"
	created, err := fixture.services.Core.Session.CreateSession(
		fixture.ownerCtx,
		sessionsvc.CreateRequest{
			SessionKey: targetSessionKey,
			AgentID:    worker.AgentID,
			Title:      "Before",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if created.ConfigurationVersion != 1 {
		t.Fatalf("new Session configuration_version=%d, want 1", created.ConfigurationVersion)
	}

	mainActor := configurationsvc.Actor{
		OwnerUserID:     fixture.main.OwnerUserID,
		AgentID:         fixture.main.AgentID,
		SessionKey:      "agent:" + fixture.main.AgentID + ":ws:dm:session-config-control",
		ContextKind:     configurationsvc.ContextKindAgent,
		ContextID:       fixture.main.AgentID,
		PrincipalRole:   authctx.RoleOwner,
		AuthMethod:      authctx.AuthMethodLocal,
		LocalSingleUser: true,
	}
	bindConfigurationTestRound(t, fixture.services, &mainActor)
	if _, err = fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		mainActor,
		[]string{configurationsvc.DomainSessions},
		true,
	); err != nil {
		t.Fatal(err)
	}
	afterInspect, err := fixture.services.Core.Session.GetMutableSession(
		fixture.ownerCtx,
		targetSessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if afterInspect.ConfigurationVersion != created.ConfigurationVersion {
		t.Fatalf(
			"Sessions inspect must be read-only: before=%d after=%d",
			created.ConfigurationVersion,
			afterInspect.ConfigurationVersion,
		)
	}

	updateInput := json.RawMessage(`{"title":"After"}`)
	updatePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		mainActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSessions, Operation: "update_title",
			Target: targetSessionKey, Input: updateInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if updatePlan.Scope.Kind != configurationsvc.ScopeKindAgent ||
		updatePlan.Scope.ID != worker.AgentID ||
		updatePlan.StateVersion != created.ConfigurationVersion ||
		!updatePlan.RequiresConfirmation {
		t.Fatalf("Session update plan 未绑定目标 Agent/version/human approval: %+v", updatePlan)
	}
	runtimeProjection := *created
	runtimeProjection.MessageCount = 1
	runtimeProjection.LastActivity = time.Now().UTC()
	runtimeUpdated, err := workspacestore.NewSessionFileStore(
		fixture.config.WorkspacePath,
	).ForOwner(worker.OwnerUserID).PatchSessionRuntime(
		worker.WorkspacePath,
		runtimeProjection,
	)
	if err != nil {
		t.Fatal(err)
	}
	if runtimeUpdated.ConfigurationVersion != updatePlan.StateVersion {
		t.Fatalf(
			"runtime patch advanced configuration_version: before=%d after=%d",
			updatePlan.StateVersion,
			runtimeUpdated.ConfigurationVersion,
		)
	}
	updateRequest := configurationsvc.ChangeRequest{
		RequestID: "session-title-update-0001",
		Domain:    configurationsvc.DomainSessions, Operation: "update_title",
		Target: targetSessionKey, Input: updateInput,
		ExpectedRevision: updatePlan.CurrentRevision, PlanDigest: updatePlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		mainActor,
		updateRequest,
		updatePlan,
	)
	updateResult, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		mainActor,
		updateRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(updateResult.Checks, "session_title_verified") ||
		!hasConfigurationCheck(updateResult.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Session 标题更新缺少写后证明: %+v", updateResult)
	}
	resultPayload, err := json.Marshal(updateResult.Result)
	if err != nil {
		t.Fatal(err)
	}
	resultText := string(resultPayload)
	for _, forbidden := range []string{
		`"session_id"`,
		`"room_session_id"`,
		`"options"`,
		`"resume_id"`,
	} {
		if strings.Contains(resultText, forbidden) {
			t.Fatalf("Session 配置结果泄漏内部字段 %s: %s", forbidden, resultText)
		}
	}
	var auditResult string
	if err = fixture.services.DB.QueryRowContext(
		fixture.ownerCtx,
		`SELECT result_json
		   FROM configuration_changes
		  WHERE owner_user_id = ? AND request_id = ?`,
		fixture.main.OwnerUserID,
		updateRequest.RequestID,
	).Scan(&auditResult); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`"session_id"`,
		`"room_session_id"`,
		`"options"`,
		`"resume_id"`,
	} {
		if strings.Contains(auditResult, forbidden) {
			t.Fatalf("Session 配置审计泄漏内部字段 %s: %s", forbidden, auditResult)
		}
	}
	updated, err := fixture.services.Core.Session.GetMutableSession(
		fixture.ownerCtx,
		targetSessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "After" ||
		updated.ConfigurationVersion != created.ConfigurationVersion+1 {
		t.Fatalf("Session 标题/CAS 未写入真相源: %+v", updated)
	}

	stalePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		mainActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSessions, Operation: "update_title",
			Target: targetSessionKey, Input: json.RawMessage(`{"title":"Stale"}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	staleRequest := configurationsvc.ChangeRequest{
		RequestID: "session-title-stale-0001",
		Domain:    configurationsvc.DomainSessions, Operation: "update_title",
		Target: targetSessionKey, Input: json.RawMessage(`{"title":"Stale"}`),
		ExpectedRevision: stalePlan.CurrentRevision, PlanDigest: stalePlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		mainActor,
		staleRequest,
		stalePlan,
	)
	if _, err = fixture.services.Core.Session.UpdateSessionTitle(
		fixture.ownerCtx,
		targetSessionKey,
		"Concurrent",
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		mainActor,
		staleRequest,
	); err == nil || !strings.Contains(err.Error(), "plan_digest") {
		t.Fatalf("并发推进后的旧 Session plan 必须失效: %v", err)
	}

	selfActor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID,
		AgentID:     worker.AgentID,
		SessionKey:  targetSessionKey,
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   worker.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &selfActor)
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		selfActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSessions, Operation: "update_title",
			Target: mainActor.SessionKey, Input: json.RawMessage(`{"title":"Forbidden"}`),
		},
	); err == nil {
		t.Fatal("普通 Agent 不得切换 target 修改其他 Session")
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		selfActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSessions, Operation: "delete",
			Target: targetSessionKey, Input: json.RawMessage(`{}`),
		},
	); err == nil {
		t.Fatal("普通 Agent 不得删除 Session")
	}
	fixture.services.Runtime.MarkRoundFinished(
		selfActor.LeaseSessionKey,
		selfActor.LeaseRoundID,
	)

	deletePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		mainActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSessions, Operation: "delete",
			Target: targetSessionKey, Input: json.RawMessage(`{}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	deleteRequest := configurationsvc.ChangeRequest{
		RequestID: "session-delete-0001",
		Domain:    configurationsvc.DomainSessions, Operation: "delete",
		Target: targetSessionKey, Input: json.RawMessage(`{}`),
		ExpectedRevision: deletePlan.CurrentRevision, PlanDigest: deletePlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		mainActor,
		deleteRequest,
		deletePlan,
	)
	deleteResult, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		mainActor,
		deleteRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(deleteResult.Checks, "configuration_target_deleted") {
		t.Fatalf("Session 删除缺少不存在证明: %+v", deleteResult)
	}
	if _, err = fixture.services.Core.Session.GetMutableSession(
		fixture.ownerCtx,
		targetSessionKey,
	); !errors.Is(err, sessionsvc.ErrSessionNotFound) {
		t.Fatalf("Session 删除后仍可读取: %v", err)
	}
}
