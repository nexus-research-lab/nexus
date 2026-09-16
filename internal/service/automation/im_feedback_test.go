package automation

import (
	"context"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	"reflect"
	"testing"
)

func TestIMFeedbackPolicyUsesExactRunAndCurrentRestrictions(t *testing.T) {
	service, db, _, task := newInitialDeliveryTestService(t, "im-feedback")
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: task.OwnerUserID, Role: authctx.RoleOwner})
	source := imdelivery.Source{Kind: "automation", AgentID: task.AgentID, JobID: task.JobID, RunID: "im-run", SessionKey: "source-session", RoundID: "source-round"}
	if err := service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{RunID: source.RunID, JobID: source.JobID, OwnerUserID: task.OwnerUserID, SessionKey: source.SessionKey, RoundID: source.RoundID, PermissionPolicyRevision: task.PermissionPolicy.Revision, Status: automationdomain.RunStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	policy, err := service.IMFeedbackPolicy(ctx, source)
	if err != nil || policy == nil || !reflect.DeepEqual(policy, taskRuntimeToolPolicy(task)) {
		t.Fatalf("policy %+v %v", policy, err)
	}
	for _, mutate := range []func(*imdelivery.Source){func(s *imdelivery.Source) { s.RoundID = "other" }, func(s *imdelivery.Source) { s.SessionKey = "other" }, func(s *imdelivery.Source) { s.AgentID = "other" }, func(s *imdelivery.Source) { s.RunID = "other" }} {
		bad := source
		mutate(&bad)
		if _, err := service.IMFeedbackPolicy(ctx, bad); err == nil {
			t.Fatal("foreign producer accepted")
		}
	}
	if _, err = db.Exec(`UPDATE automation_task_runs SET permission_policy_revision=permission_policy_revision+1 WHERE run_id=?`, source.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.IMFeedbackPolicy(ctx, source); err == nil {
		t.Fatal("stale policy accepted")
	}
	run, err := service.repository.GetRun(ctx, task.OwnerUserID, task.JobID, source.RunID)
	if err != nil || run.Status != automationdomain.RunStatusSucceeded {
		t.Fatalf("feedback changed old run %+v %v", run, err)
	}
}
