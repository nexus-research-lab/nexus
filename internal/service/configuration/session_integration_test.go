// INPUT: owner-main/agent-self 可信 DM Actor、workspace Session 与人类批准。
// OUTPUT: Session inspect/plan/CAS/apply/verify、越权拒绝、过期计划拒绝与安全删除证明。
// POS: nexuscfg Sessions 域跨配置、runtime 与 owner-confined 文件真相源的端到端回归。
package configuration_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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
