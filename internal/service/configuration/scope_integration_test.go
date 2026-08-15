package configuration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

type scopedConfigurationFixture struct {
	services *server.AppServices
	ownerCtx context.Context
	main     *protocol.Agent
	config   config.Config
}

type recordingRoomRuntime struct {
	mu      sync.Mutex
	agentID string
	mode    sdkpermission.Mode
}

func (r *recordingRoomRuntime) SetPermissionModeForAgent(
	_ context.Context,
	agentID string,
	mode sdkpermission.Mode,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agentID = agentID
	r.mode = mode
	return nil
}

func (*recordingRoomRuntime) InterruptAgentTasks(context.Context, string, string, string) error {
	return nil
}

func newScopedConfigurationFixture(t *testing.T) scopedConfigurationFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, "state"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "config"))
	cfg := config.Config{
		DatabaseDriver:          "sqlite",
		DatabaseURL:             filepath.Join(root, "nexus.db"),
		DefaultAgentID:          "nexus",
		WorkspacePath:           filepath.Join(root, "workspace"),
		DefaultTimezone:         "Asia/Shanghai",
		ConnectorCredentialsKey: "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	services := server.NewAppServicesWithDB(cfg, db, nil)
	enableConfigurationTestPrincipalVerification(services)
	if err = services.Core.Agent.EnsureReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	mainAgent, err := services.Core.Agent.GetDefaultAgent(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ownerCtx := authctx.WithPrincipal(t.Context(), &authctx.Principal{
		UserID: mainAgent.OwnerUserID, Username: mainAgent.AgentID,
		Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodLocal,
	})
	return scopedConfigurationFixture{
		services: services,
		ownerCtx: ownerCtx,
		main:     mainAgent,
		config:   cfg,
	}
}

func (f scopedConfigurationFixture) createAgent(t *testing.T, name string) *protocol.Agent {
	t.Helper()
	item, err := f.services.Core.Agent.CreateAgent(f.ownerCtx, protocol.CreateRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func skillSelectionInput(
	t *testing.T,
	services *server.AppServices,
	agentID string,
	skillName string,
	targetScope skillsvc.AgentSkillTargetScope,
	includeAgentID bool,
) json.RawMessage {
	t.Helper()
	state, err := services.Skills.GetAgentSkillStateInScope(
		t.Context(),
		agentID,
		skillName,
		targetScope,
	)
	if err != nil {
		t.Fatalf("读取 Skill 来源身份失败: %v", err)
	}
	input := map[string]any{
		"target_scope":    targetScope,
		"source_identity": state.SourceIdentity,
	}
	if includeAgentID {
		input["agent_id"] = agentID
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestConfigurationRejectsMissingOrForgedTrustedContext(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Context Boundary Worker")
	cases := []configurationsvc.Actor{
		{
			OwnerUserID: fixture.main.OwnerUserID,
			AgentID:     fixture.main.AgentID,
			IsMainAgent: true,
		},
		{
			OwnerUserID: worker.OwnerUserID,
			AgentID:     worker.AgentID,
			ContextKind: configurationsvc.ContextKindAgent,
		},
		{
			OwnerUserID: worker.OwnerUserID,
			AgentID:     worker.AgentID,
			ContextKind: configurationsvc.ContextKindAgent,
			ContextID:   fixture.main.AgentID,
		},
	}
	for _, actor := range cases {
		if _, err := fixture.services.Configuration.Inspect(
			fixture.ownerCtx, actor, []string{configurationsvc.DomainAgents}, false,
		); err == nil {
			t.Fatalf("untrusted context must be rejected: %+v", actor)
		}
	}

	originalActor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID,
		AgentID:     worker.AgentID,
		SessionKey:  "agent:" + worker.AgentID + ":dm:first",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   worker.AgentID,
	}
	input := json.RawMessage(`{"description":"session-bound"}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		originalActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "update_self_profile", Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	replayedActor := originalActor
	replayedActor.SessionKey = "agent:" + worker.AgentID + ":dm:second"
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		replayedActor,
		configurationsvc.ChangeRequest{
			RequestID: "cross-session-plan-01",
			Domain:    configurationsvc.DomainAgents, Operation: "update_self_profile",
			Input:            input,
			ExpectedRevision: plan.CurrentRevision,
			PlanDigest:       plan.PlanDigest,
		},
	); err == nil {
		t.Fatal("configuration plan must not be replayed from another runtime session")
	}
}

func TestMainAgentCannotAcquireRoomConfigurationAuthority(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	member := fixture.createAgent(t, "Main Boundary Member")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{AgentIDs: []string{member.AgentID}},
	)
	if err != nil {
		t.Fatal(err)
	}
	actor := configurationsvc.Actor{
		OwnerUserID:    fixture.main.OwnerUserID,
		AgentID:        fixture.main.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomContext.Room.ID,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID,
			fixture.main.AgentID,
			roomContext.Room.RoomType,
		),
	}
	if _, err = fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainRooms},
		false,
	); err == nil || !strings.Contains(err.Error(), "主智能体不能作为 Group Room 成员") {
		t.Fatalf("主智能体 Room 配置边界 error = %v", err)
	}
}

func TestAgentSelfConfigurationBoundaryAndResourceCAS(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Self Config Worker")
	other := fixture.createAgent(t, "Other Worker")
	actor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID, AgentID: worker.AgentID,
		SessionKey:  "agent:" + worker.AgentID + ":ws:dm:main",
		ContextKind: configurationsvc.ContextKindAgent, ContextID: worker.AgentID,
	}

	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainAgents, configurationsvc.DomainProviders, configurationsvc.DomainSkills},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	agentSnapshot := inspection.Domains[configurationsvc.DomainAgents]
	if inspection.Authority != configurationsvc.AuthorityAgentSelf ||
		agentSnapshot.Scope.ID != worker.AgentID ||
		len(agentSnapshot.Access.AllowedOperations) != 2 {
		t.Fatalf("unexpected self access: %+v", inspection)
	}
	providerPayload, err := json.Marshal(inspection.Domains[configurationsvc.DomainProviders].Values)
	if err != nil {
		t.Fatal(err)
	}
	providerText := string(providerPayload)
	if !strings.Contains(providerText, `"catalog"`) {
		t.Fatalf("self provider snapshot missing safe model catalog: %s", providerText)
	}
	for _, forbidden := range []string{
		`"base_url"`, `"auth_token_masked"`, `"last_test_error"`, `"used_by_agents"`, `"models_path"`,
	} {
		if strings.Contains(providerText, forbidden) {
			t.Fatalf("self provider snapshot exposed owner-only field %s: %s", forbidden, providerText)
		}
	}
	if _, err = fixture.services.Configuration.Inspect(
		fixture.ownerCtx, actor, []string{configurationsvc.DomainPreferences}, false,
	); err == nil {
		t.Fatal("ordinary Agent must not read owner preferences")
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "update_self_profile",
			Target: other.AgentID, Input: json.RawMessage(`{"description":"forged"}`),
		},
	); err == nil {
		t.Fatal("ordinary Agent must not target another Agent")
	}

	maxTurns := 8
	options := worker.Options
	options.MaxTurns = &maxTurns
	if _, err = fixture.services.Core.Agent.UpdateAgent(
		fixture.ownerCtx,
		worker.AgentID,
		protocol.UpdateRequest{Options: &options},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "update_self_runtime",
			Input: json.RawMessage(`{"max_turns":9}`),
		},
	); err == nil || !strings.Contains(err.Error(), "只能收紧") {
		t.Fatalf("ordinary Agent must not raise its execution ceiling: %v", err)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "update_self_runtime",
			Input: json.RawMessage(`{"max_turns":4}`),
		},
	); err != nil {
		t.Fatalf("ordinary Agent should be able to tighten its execution ceiling: %v", err)
	}

	bindConfigurationTestRound(t, fixture.services, &actor)
	plans := make([]*configurationsvc.ChangePlan, 2)
	for index, description := range []string{"first profile", "second profile"} {
		plans[index], err = fixture.services.Configuration.PlanChange(
			fixture.ownerCtx,
			actor,
			configurationsvc.ChangeRequest{
				Domain: configurationsvc.DomainAgents, Operation: "update_self_profile",
				Input: json.RawMessage(`{"description":"` + description + `"}`),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	if plans[0].StateVersion == 0 || plans[0].StateVersion != plans[1].StateVersion {
		t.Fatalf("plans did not bind one Agent resource version: %+v", plans)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			RequestID: "self-no-confirm-01", Domain: configurationsvc.DomainAgents,
			Operation: "update_self_profile", Input: json.RawMessage(`{"description":"first profile"}`),
			ExpectedRevision: plans[0].CurrentRevision, PlanDigest: plans[0].PlanDigest,
		},
	); err == nil {
		t.Fatal("self profile identity change must require explicit confirmation")
	}

	results := make(chan error, 2)
	var wait sync.WaitGroup
	requests := make([]configurationsvc.ChangeRequest, 2)
	for index, description := range []string{"first profile", "second profile"} {
		requests[index] = configurationsvc.ChangeRequest{
			RequestID: "self-concurrent-0" + string(rune('1'+index)),
			Domain:    configurationsvc.DomainAgents, Operation: "update_self_profile",
			Input:            json.RawMessage(`{"description":"` + description + `"}`),
			ExpectedRevision: plans[index].CurrentRevision,
			PlanDigest:       plans[index].PlanDigest,
		}
		approveConfigurationTestChange(
			t,
			fixture.services,
			fixture.ownerCtx,
			actor,
			requests[index],
			plans[index],
		)
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, applyErr := fixture.services.Configuration.ApplyChange(
				fixture.ownerCtx,
				actor,
				requests[index],
			)
			results <- applyErr
		}(index)
	}
	wait.Wait()
	close(results)
	successes := 0
	for applyErr := range results {
		if applyErr == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent writes succeeded %d times, want exactly one", successes)
	}
	updated, err := fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, worker.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RuntimeVersion <= worker.RuntimeVersion {
		t.Fatalf("runtime_version did not advance: before=%d after=%d", worker.RuntimeVersion, updated.RuntimeVersion)
	}
}

func TestAgentSelfCanApplyOwnProfileThroughNexuscfg(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "CLI Self Config Worker")
	actor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID,
		AgentID:     worker.AgentID,
		SessionKey:  "agent:" + worker.AgentID + ":ws:dm:main",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   worker.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	input := json.RawMessage(`{"description":"updated by nexuscfg"}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "update_self_profile", Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.services.Configuration.ApplyChangeFromCLI(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			RequestID: "agent-self-cli-0001", Domain: configurationsvc.DomainAgents,
			Operation: "update_self_profile", Input: input,
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		configurationsvc.CLIApplyOptions{Confirmed: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied {
		t.Fatalf("nexuscfg apply result = %+v", result)
	}
	updated, err := fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, worker.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "updated by nexuscfg" {
		t.Fatalf("ordinary Agent profile description = %q", updated.Description)
	}
}

func TestSkillChangesBindSelfAndMainPlansToTargetAgentCAS(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	mainTarget := fixture.createAgent(t, "Main Skill Target")
	selfTarget := fixture.createAgent(t, "Self Skill Target")
	mainActor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID, AgentID: fixture.main.AgentID,
		IsMainAgent: true, SessionKey: "agent:" + fixture.main.AgentID + ":ws:dm:skills",
		ContextKind: configurationsvc.ContextKindAgent, ContextID: fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &mainActor)
	mainInput := skillSelectionInput(
		t,
		fixture.services,
		mainTarget.AgentID,
		"ima-skill",
		skillsvc.AgentSkillTargetGlobalLibrary,
		true,
	)
	mainPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		mainActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "install",
			Target: "ima-skill", Input: mainInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mainPlan.Scope.Kind != configurationsvc.ScopeKindAgent ||
		mainPlan.Scope.ID != mainTarget.AgentID ||
		mainPlan.StateVersion != mainTarget.RuntimeVersion {
		t.Fatalf("owner-main Skills plan 未绑定目标 Agent: %+v", mainPlan)
	}
	planPayload, err := json.Marshal(mainPlan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(planPayload), `"target_scope":"global_library"`) ||
		!strings.Contains(string(planPayload), `"source_identity":"skill-source:`) {
		t.Fatalf("Skills plan 未显式返回 target_scope/source_identity: %s", planPayload)
	}
	mainRequest := configurationsvc.ChangeRequest{
		RequestID: "main-skill-install-01",
		Domain:    configurationsvc.DomainSkills, Operation: "install",
		Target: "ima-skill", Input: mainInput,
		ExpectedRevision: mainPlan.CurrentRevision, PlanDigest: mainPlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		mainActor,
		mainRequest,
		mainPlan,
	)
	mainResult, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		mainActor,
		mainRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if mainResult.Scope.Kind != configurationsvc.ScopeKindAgent ||
		mainResult.Scope.ID != mainTarget.AgentID ||
		!hasConfigurationCheck(mainResult.Checks, "configuration_resource_version_advanced") ||
		!hasConfigurationCheck(mainResult.Checks, "skill_target_installation_state_verified") {
		t.Fatalf("owner-main Skills apply 未完成 Agent CAS 核对: %+v", mainResult)
	}
	installedByMain, err := fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, mainTarget.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if installedByMain.RuntimeVersion != mainTarget.RuntimeVersion+1 ||
		!slices.Contains(installedByMain.Options.SkillIDs, "ima-skill") {
		t.Fatalf("owner-main Skills 安装未写入目标 Agent: %+v", installedByMain)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		mainActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "install",
			Target: "ima-skill", Input: mainInput,
		},
	); err == nil {
		t.Fatal("已启用 Skill 的重复 plan 必须被拒绝")
	}

	staleUninstallPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		mainActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "uninstall",
			Target: "ima-skill", Input: mainInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	concurrentOptions := installedByMain.Options
	concurrentOptions.AllowedTools = []string{"Read"}
	expectedVersion := installedByMain.RuntimeVersion
	concurrentlyUpdated, err := fixture.services.Core.Agent.UpdateAgent(
		fixture.ownerCtx,
		mainTarget.AgentID,
		protocol.UpdateRequest{
			Options: &concurrentOptions, ExpectedRuntimeVersion: &expectedVersion,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		mainActor,
		configurationsvc.ChangeRequest{
			RequestID: "main-skill-stale-01",
			Domain:    configurationsvc.DomainSkills, Operation: "uninstall",
			Target: "ima-skill", Input: mainInput,
			ExpectedRevision: staleUninstallPlan.CurrentRevision,
			PlanDigest:       staleUninstallPlan.PlanDigest,
		},
	); err == nil {
		t.Fatal("并发 Agent options 更新后，过期 Skills plan 必须失败")
	}
	afterConflict, err := fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, mainTarget.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if afterConflict.RuntimeVersion != concurrentlyUpdated.RuntimeVersion ||
		!slices.Equal(afterConflict.Options.AllowedTools, []string{"Read"}) ||
		!slices.Contains(afterConflict.Options.SkillIDs, "ima-skill") {
		t.Fatalf("过期 Skills plan 覆盖了并发 Agent options: %+v", afterConflict)
	}

	selfActor := configurationsvc.Actor{
		OwnerUserID: selfTarget.OwnerUserID, AgentID: selfTarget.AgentID,
		SessionKey:  "agent:" + selfTarget.AgentID + ":ws:dm:skills",
		ContextKind: configurationsvc.ContextKindAgent, ContextID: selfTarget.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &selfActor)
	selfInput := skillSelectionInput(
		t,
		fixture.services,
		selfTarget.AgentID,
		"ima-skill",
		skillsvc.AgentSkillTargetGlobalLibrary,
		false,
	)
	selfPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		selfActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "install_self",
			Target: "ima-skill", Input: selfInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if selfPlan.Scope.Kind != configurationsvc.ScopeKindAgent ||
		selfPlan.Scope.ID != selfTarget.AgentID ||
		selfPlan.StateVersion != selfTarget.RuntimeVersion {
		t.Fatalf("agent-self Skills plan 未绑定自身 Agent: %+v", selfPlan)
	}
	selfRequest := configurationsvc.ChangeRequest{
		RequestID: "self-skill-install-01",
		Domain:    configurationsvc.DomainSkills, Operation: "install_self",
		Target: "ima-skill", Input: selfInput,
		ExpectedRevision: selfPlan.CurrentRevision,
		PlanDigest:       selfPlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		selfActor,
		selfRequest,
		selfPlan,
	)
	selfResult, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		selfActor,
		selfRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if selfResult.Scope.ID != selfTarget.AgentID ||
		!hasConfigurationCheck(selfResult.Checks, "configuration_resource_version_advanced") ||
		!hasConfigurationCheck(selfResult.Checks, "skill_target_installation_state_verified") {
		t.Fatalf("agent-self Skills apply 未完成自身 CAS 核对: %+v", selfResult)
	}
	installedBySelf, err := fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, selfTarget.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if installedBySelf.RuntimeVersion != selfTarget.RuntimeVersion+1 ||
		!slices.Contains(installedBySelf.Options.SkillIDs, "ima-skill") {
		t.Fatalf("agent-self Skills 安装未写入自身 Agent: %+v", installedBySelf)
	}
}

func TestConnectorConversationChangeBindsTargetVersionAndRejectsStaleSecretOverwrite(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		IsMainAgent: true,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:connectors",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	const conversationSecret = "conversation-amap-secret"
	input := json.RawMessage(`{
		"credentials":{"api_key":{"$secret":"connector.api_key"}}
	}`)
	stalePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainConnectors, Operation: "connect",
			Target: "amap", Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if stalePlan.StateVersion != 1 || stalePlan.Scope.Kind != configurationsvc.ScopeKindOwner {
		t.Fatalf("Connector plan 未绑定目标版本: %+v", stalePlan)
	}

	if _, err = fixture.services.Connectors.Connect(
		fixture.ownerCtx,
		actor.OwnerUserID,
		"amap",
		map[string]string{"api_key": "newer-http-secret"},
	); err != nil {
		t.Fatalf("模拟 HTTP 并发写失败: %v", err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			RequestID: "connector-stale-apply-01",
			Domain:    configurationsvc.DomainConnectors, Operation: "connect",
			Target: "amap", Input: input, ExpectedRevision: stalePlan.CurrentRevision,
			PlanDigest: stalePlan.PlanDigest,
		},
	); err == nil {
		t.Fatal("Connector 并发变更后旧对话计划必须失败")
	}
	current, err := fixture.services.Connectors.LoadActiveConnection(
		fixture.ownerCtx,
		actor.OwnerUserID,
		"amap",
	)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.AccessToken != "newer-http-secret" {
		t.Fatalf("旧计划覆盖了较新的 Connector 凭据: %+v", current)
	}

	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainConnectors, Operation: "connect",
			Target: "amap", Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: "connector-current-apply-01",
		Domain:    configurationsvc.DomainConnectors, Operation: "connect",
		Target: "amap", Input: input, ExpectedRevision: plan.CurrentRevision,
		PlanDigest: plan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		request,
		plan,
		map[string]string{"connector.api_key": conversationSecret},
	)
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") ||
		!hasConfigurationCheck(result.Checks, "connector_target_outcome_verified") {
		t.Fatalf("Connector apply 缺少 CAS 或写后核验: %+v", result)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), conversationSecret) {
		t.Fatalf("Connector apply 泄漏 secret: %s", payload)
	}
}

func TestConnectorConversationChangeIgnoresUnrelatedConnectorMutation(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		IsMainAgent: true,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:connector-target",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	const targetSecret = "target-amap-secret"
	input := json.RawMessage(`{
		"credentials":{"api_key":{"$secret":"connector.api_key"}}
	}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainConnectors, Operation: "connect",
			Target: "amap", Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = fixture.services.Connectors.Connect(
		fixture.ownerCtx,
		actor.OwnerUserID,
		"didi",
		map[string]string{"api_key": "unrelated-didi-secret"},
	); err != nil {
		t.Fatalf("修改无关 Connector 失败: %v", err)
	}
	currentPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainConnectors, Operation: "connect",
			Target: "amap", Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if currentPlan.CurrentRevision != plan.CurrentRevision ||
		currentPlan.StateVersion != plan.StateVersion ||
		currentPlan.PlanDigest != plan.PlanDigest {
		t.Fatalf(
			"无关 Connector 变化不应使目标计划失效: before=%+v after=%+v",
			plan,
			currentPlan,
		)
	}

	request := configurationsvc.ChangeRequest{
		RequestID: "connector-target-only-apply-01",
		Domain:    configurationsvc.DomainConnectors, Operation: "connect",
		Target: "amap", Input: input, ExpectedRevision: plan.CurrentRevision,
		PlanDigest: plan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		request,
		plan,
		map[string]string{"connector.api_key": targetSecret},
	)
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") ||
		!hasConfigurationCheck(result.Checks, "connector_target_outcome_verified") {
		t.Fatalf("目标 Connector apply 缺少版本与写后核验: %+v", result)
	}
}

func TestWorkspaceSkillConversationDisablePreservesFilesAndVerifiesTargetState(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Workspace Skill Target")
	const skillName = "workspace-state-skill"
	skillRoot := filepath.Join(worker.WorkspacePath, ".agents", "skills", skillName)
	if err := os.MkdirAll(skillRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte(`---
name: workspace-state-skill
title: Workspace State Skill
description: workspace uninstall verification
---

# Workspace State Skill
`), 0o644); err != nil {
		t.Fatal(err)
	}
	workspaceState, err := fixture.services.Skills.GetAgentSkillStateInScope(
		t.Context(),
		worker.AgentID,
		skillName,
		skillsvc.AgentSkillTargetWorkspace,
	)
	if err != nil {
		t.Fatal(err)
	}
	mainActor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID, AgentID: fixture.main.AgentID,
		IsMainAgent: true, SessionKey: "agent:" + fixture.main.AgentID + ":dm:workspace-skills",
		ContextKind: configurationsvc.ContextKindAgent, ContextID: fixture.main.AgentID,
	}
	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		mainActor,
		[]string{configurationsvc.DomainSkills},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	inspectionPayload, err := json.Marshal(inspection.Domains[configurationsvc.DomainSkills].Values)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(inspectionPayload), `"agent_workspace_items"`) ||
		!strings.Contains(string(inspectionPayload), worker.AgentID) ||
		!strings.Contains(string(inspectionPayload), workspaceState.SourceIdentity) {
		t.Fatalf("owner-main inspect 未暴露目标 Agent workspace 来源身份: %s", inspectionPayload)
	}
	actor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID, AgentID: worker.AgentID,
		SessionKey:  "agent:" + worker.AgentID + ":ws:dm:workspace-skill",
		ContextKind: configurationsvc.ContextKindAgent, ContextID: worker.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	workspaceInput := skillSelectionInput(
		t,
		fixture.services,
		worker.AgentID,
		skillName,
		skillsvc.AgentSkillTargetWorkspace,
		false,
	)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "uninstall_self",
			Target: skillName, Input: workspaceInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope.Kind != configurationsvc.ScopeKindAgent ||
		plan.Scope.ID != worker.AgentID ||
		plan.StateVersion != worker.RuntimeVersion {
		t.Fatalf("workspace Skill plan 未绑定目标 Agent: %+v", plan)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: "workspace-skill-remove-01",
		Domain:    configurationsvc.DomainSkills, Operation: "uninstall_self",
		Target: skillName, Input: workspaceInput,
		ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	}
	approveConfigurationTestChange(t, fixture.services, fixture.ownerCtx, actor, request, plan)
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(result.Checks, "skill_target_installation_state_verified") ||
		!hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("workspace Skill 卸载缺少写后状态证明: %+v", result)
	}
	if _, err = os.Stat(skillRoot); err != nil {
		t.Fatalf("workspace Skill 对话停用不得删除目录: %v", err)
	}
	updated, err := fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, worker.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RuntimeVersion != worker.RuntimeVersion+1 {
		t.Fatalf("workspace Skill 卸载 runtime_version=%d, want %d", updated.RuntimeVersion, worker.RuntimeVersion+1)
	}
	if !slices.Contains(updated.Options.DisabledSkillIDs, skillName) {
		t.Fatalf("workspace Skill 对话停用未写入 disabled_skill_ids: %#v", updated.Options.DisabledSkillIDs)
	}
}

func TestRoomHostConfigurationBoundaryAndImmediateAuthorityRevocation(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Room Host")
	member := fixture.createAgent(t, "Room Member")
	roomContext, err := fixture.services.Core.Room.CreateRoom(fixture.ownerCtx, protocol.CreateRoomRequest{
		AgentIDs: []string{host.AgentID, member.AgentID},
		Name:     "Scoped Room", HostAgentID: host.AgentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	hostActor := configurationsvc.Actor{
		OwnerUserID: host.OwnerUserID, AgentID: host.AgentID,
		ContextKind: configurationsvc.ContextKindRoom, ContextID: roomContext.Room.ID,
		RoomID: roomContext.Room.ID, ConversationID: roomContext.Conversation.ID,
		SessionKey: protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType,
		),
	}
	memberActor := configurationsvc.Actor{
		OwnerUserID: member.OwnerUserID, AgentID: member.AgentID,
		ContextKind: configurationsvc.ContextKindRoom, ContextID: roomContext.Room.ID,
		RoomID: roomContext.Room.ID, ConversationID: roomContext.Conversation.ID,
		SessionKey: protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID, member.AgentID, roomContext.Room.RoomType,
		),
	}

	memberInspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx, memberActor, []string{configurationsvc.DomainRooms}, false,
	)
	if err != nil {
		t.Fatal(err)
	}
	memberRoom := memberInspection.Domains[configurationsvc.DomainRooms]
	if memberInspection.Authority != configurationsvc.AuthorityRoomMember ||
		!memberRoom.Access.CanRead ||
		len(memberRoom.Access.AllowedOperations) != 0 {
		t.Fatalf("unexpected member access: %+v", memberInspection)
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		memberActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "update_profile",
			Input: json.RawMessage(`{"description":"forged"}`),
		},
	); err == nil {
		t.Fatal("ordinary Room member must not mutate shared settings")
	}
	if _, err = fixture.services.Configuration.Inspect(
		fixture.ownerCtx, hostActor, []string{configurationsvc.DomainAgents}, false,
	); err == nil {
		t.Fatal("Room host must not use Room context to read Agent global configuration")
	}
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "update_profile",
			Input: json.RawMessage(`{"title":"must-bind-a-conversation"}`),
		},
	); err == nil {
		t.Fatal("Room profile configuration must not mutate an unbound conversation title")
	}

	stalePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "set_collaboration_policy",
			Input: json.RawMessage(`{"private_messages_enabled":true}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	newHostID := member.AgentID
	transferred, err := fixture.services.Core.Room.UpdateRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
		protocol.UpdateRoomRequest{HostAgentID: &newHostID},
	)
	if err != nil {
		t.Fatal(err)
	}
	if transferred.Room.AuthorityEpoch <= roomContext.Room.AuthorityEpoch {
		t.Fatal("host transfer did not advance authority_epoch")
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		hostActor,
		configurationsvc.ChangeRequest{
			RequestID: "stale-room-host-01", Domain: configurationsvc.DomainRooms,
			Operation:        "set_collaboration_policy",
			Input:            json.RawMessage(`{"private_messages_enabled":true}`),
			ExpectedRevision: stalePlan.CurrentRevision, PlanDigest: stalePlan.PlanDigest,
		},
	); err == nil {
		t.Fatal("old host authority must be revoked before apply")
	}

	newHostActor := memberActor
	bindConfigurationTestRound(t, fixture.services, &newHostActor)
	policyPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		newHostActor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "set_collaboration_policy",
			Input: json.RawMessage(`{"private_messages_enabled":true,"host_auto_reply_enabled":true}`),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	policyRequest := configurationsvc.ChangeRequest{
		RequestID: "new-room-host-001", Domain: configurationsvc.DomainRooms,
		Operation:        "set_collaboration_policy",
		Input:            json.RawMessage(`{"private_messages_enabled":true,"host_auto_reply_enabled":true}`),
		ExpectedRevision: policyPlan.CurrentRevision, PlanDigest: policyPlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		newHostActor,
		policyRequest,
		policyPlan,
	)
	applied, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		newHostActor,
		policyRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Reload.CurrentRoundAffected || applied.Scope.ID != roomContext.Room.ID {
		t.Fatalf("Room hot reload result is incomplete: %+v", applied)
	}
	if !hasConfigurationCheck(applied.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Room update did not prove its resource version transition: %+v", applied.Checks)
	}
	current, err := fixture.services.Core.Room.GetRoom(fixture.ownerCtx, roomContext.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !current.Room.PrivateMessagesEnabled || !current.Room.HostAutoReplyEnabled ||
		current.Room.ConfigurationVersion <= transferred.Room.ConfigurationVersion {
		t.Fatalf("Room collaboration policy did not persist: %+v", current.Room)
	}
}

func TestRoomHostParticipationChangeUsesCASAndImmediateAuthorityFence(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Participation Host")
	member := fixture.createAgent(t, "Participation Member")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs:    []string{host.AgentID, member.AgentID},
			HostAgentID: host.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	actor := configurationsvc.Actor{
		OwnerUserID:    host.OwnerUserID,
		AgentID:        host.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomContext.Room.ID,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID,
			host.AgentID,
			roomContext.Room.RoomType,
		),
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	input := json.RawMessage(`{"agent_id":"` + member.AgentID + `","paused":true}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "set_member_participation",
			Input:     input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID:        "room-participation-001",
		Domain:           configurationsvc.DomainRooms,
		Operation:        "set_member_participation",
		Input:            input,
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		request,
		plan,
	)
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reload.CurrentRoundAffected ||
		!hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") ||
		!hasConfigurationCheck(result.Checks, "room_member_participation_verified") {
		t.Fatalf("Room participation apply 缺少即时生效或 CAS 证明: %+v", result)
	}
	current, err := fixture.services.Core.Room.GetRoom(fixture.ownerCtx, roomContext.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Room.ConfigurationVersion != roomContext.Room.ConfigurationVersion+1 ||
		current.Room.AuthorityEpoch != roomContext.Room.AuthorityEpoch+1 {
		t.Fatalf("Room participation 未推进配置版本和权限世代: before=%+v after=%+v", roomContext.Room, current.Room)
	}
	paused := false
	for _, roomMember := range current.Members {
		if roomMember.MemberAgentID == member.AgentID {
			paused = roomMember.ParticipationPaused
		}
	}
	if !paused {
		t.Fatalf("Room member participation 未持久暂停: %+v", current.Members)
	}

	selfPauseInput := json.RawMessage(`{"agent_id":"` + host.AgentID + `","paused":true}`)
	if _, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainRooms,
			Operation: "set_member_participation",
			Input:     selfPauseInput,
		},
	); err == nil {
		t.Fatal("Room host 不应通过自己的 Room round 暂停自己并造成对话控制锁死")
	}
}

func TestRoomHostHumanApprovalIsRevokedByHostTransfer(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	host := fixture.createAgent(t, "Approval Host")
	nextHost := fixture.createAgent(t, "Approval Next Host")
	roomContext, err := fixture.services.Core.Room.CreateRoom(
		fixture.ownerCtx,
		protocol.CreateRoomRequest{
			AgentIDs:    []string{host.AgentID, nextHost.AgentID},
			Name:        "Approval Revocation Room",
			HostAgentID: host.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	actor := configurationsvc.Actor{
		OwnerUserID: host.OwnerUserID, AgentID: host.AgentID,
		ContextKind:    configurationsvc.ContextKindRoom,
		ContextID:      roomContext.Room.ID,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		SessionKey:     protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType,
		),
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	input := json.RawMessage(`{"private_messages_enabled":true}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainRooms, Operation: "set_collaboration_policy",
			Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: "room-host-revoked-approval-01",
		Domain:    configurationsvc.DomainRooms, Operation: "set_collaboration_policy",
		Input: input, ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		request,
		plan,
	)
	newHostID := nextHost.AgentID
	if _, err = fixture.services.Core.Room.UpdateRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
		protocol.UpdateRoomRequest{HostAgentID: &newHostID},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	); err == nil {
		t.Fatal("old Room host used a previously granted human approval after host transfer")
	}
	current, err := fixture.services.Core.Room.GetRoom(
		fixture.ownerCtx,
		roomContext.Room.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if current.Room.PrivateMessagesEnabled {
		t.Fatal("revoked Room host approval changed collaboration policy")
	}
}

func TestMainAgentPermissionChangeHotReloadsActiveRuntimeBoundary(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Permission Reload Worker")
	recorder := &recordingRoomRuntime{}
	fixture.services.Configuration.SetRoomControl(fixture.services.Core.Room, recorder)
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID, AgentID: fixture.main.AgentID,
		IsMainAgent: true, ContextKind: configurationsvc.ContextKindAgent,
		ContextID: fixture.main.AgentID, SessionKey: "agent:nexus:ws:dm:main",
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	input := json.RawMessage(`{"options":{"permission_mode":"plan"}}`)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "update",
			Target: worker.AgentID, Input: input,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: "permission-reload-01", Domain: configurationsvc.DomainAgents,
		Operation: "update", Target: worker.AgentID, Input: input,
		ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	}
	approveConfigurationTestChange(t, fixture.services, fixture.ownerCtx, actor, request, plan)
	applied, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder.mu.Lock()
	gotAgentID, gotMode := recorder.agentID, recorder.mode
	recorder.mu.Unlock()
	if gotAgentID != worker.AgentID || gotMode != sdkpermission.ModePlan {
		t.Fatalf("Room runtime hot reload = (%q, %q)", gotAgentID, gotMode)
	}
	if !applied.Reload.CurrentRoundAffected {
		t.Fatalf("permission hot reload was not reported: %+v", applied.Reload)
	}
	if !hasConfigurationCheck(applied.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Agent update did not prove its resource version transition: %+v", applied.Checks)
	}
}

func TestMainAgentDeleteVerifiesTargetAbsence(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Delete Verification Worker")
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		IsMainAgent: true,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:delete",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainAgents, Operation: "delete", Target: worker.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: "delete-agent-verify-01",
		Domain:    configurationsvc.DomainAgents, Operation: "delete", Target: worker.AgentID,
		ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	}
	approveConfigurationTestChange(t, fixture.services, fixture.ownerCtx, actor, request, plan)
	applied, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(applied.Checks, "configuration_target_deleted") {
		t.Fatalf("Agent deletion was not verified against source of truth: %+v", applied)
	}
	if _, err = fixture.services.Core.Agent.GetAgent(fixture.ownerCtx, worker.AgentID); err == nil {
		t.Fatal("Agent still exists after verified delete")
	}
}
