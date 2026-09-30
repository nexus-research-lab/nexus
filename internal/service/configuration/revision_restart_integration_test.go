// INPUT: 持久配置与 unknown receipt、数据库关闭重开及新的配置服务实例。
// OUTPUT: 重启后 revision 可比较、旧批准摘要失效、人工对账不重放写入的证明。
// POS: 配置恢复链路的数据库重启集成回归。
package configuration_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

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
