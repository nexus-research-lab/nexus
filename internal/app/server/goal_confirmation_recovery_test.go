package server

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestGoalConfirmationLifecycleRecoveryConvergesPendingReceipt(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	const (
		goalID      = "goal-lifecycle-recovery"
		executionID = "execution-lifecycle-recovery"
		sessionKey  = "agent:nexus:workspace:dm:goal-lifecycle-recovery"
	)
	if _, err = db.Exec(`
INSERT INTO session_goals (
    goal_id, session_key, objective, status, version, metadata_json
) VALUES (?, ?, 'recover Goal confirmation', 'active', 1, ?)`,
		goalID,
		sessionKey,
		`{"objective_revision":1,"execution_id":"`+executionID+
			`","execution_binding_state":"pending","activation_origin":"user_explicit",`+
			`"activation_reason":"persistence_requested","completion_criteria":["verified"]}`,
	); err != nil {
		t.Fatal(err)
	}
	store := orchestrationstore.NewRepository(cfg, db)
	if _, err = store.Create(context.Background(), orchestrationstore.CreateCommand{
		Execution: protocol.Execution{
			ID:                    executionID,
			OwnerUserID:           "owner-1",
			SessionKey:            sessionKey,
			ScopeKind:             protocol.ExecutionScopeDM,
			CoordinatorAgentID:    "agent-lead",
			Origin:                protocol.ExecutionOriginUserRequest,
			Objective:             "recover Goal confirmation",
			CompletionCriteria:    []string{"verified"},
			GoalID:                goalID,
			GoalObjectiveRevision: 1,
			GoalActivationOrigin:  protocol.GoalActivationOriginUserExplicit,
			GoalActivationReason:  protocol.GoalActivationReasonPersistenceRequested,
			Status:                protocol.ExecutionStatusActive,
		},
		Meta: orchestrationstore.CommandMeta{
			CommandID: "create-lifecycle-recovery",
			EventID:   "event-lifecycle-recovery",
			ActorKind: protocol.ExecutionActorSystem,
			ActorID:   "server",
		},
	}); err != nil {
		t.Fatal(err)
	}

	services := NewAppServicesWithDB(cfg, db, logx.NewDiscardLogger())
	server := &Server{
		api:      handlershared.NewAPI(logx.NewDiscardLogger()),
		services: services,
	}
	if _, deadlineErr := store.OrchestrationRecoveryDeadlines(context.Background()); deadlineErr != nil {
		t.Fatalf("read recovery deadlines: %v", deadlineErr)
	}
	stop, err := server.startOrchestrationRecovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	waitForServerCondition(t, func() bool {
		current, getErr := store.GetGoalConfirmationReceipt(context.Background(), executionID)
		return getErr == nil && current != nil &&
			current.State == orchestrationstore.GoalConfirmationConfirmed
	})
	if stop != nil {
		stop()
	}
	receipt, err := store.GetGoalConfirmationReceipt(context.Background(), executionID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.State != orchestrationstore.GoalConfirmationConfirmed {
		t.Fatalf("lifecycle recovery receipt = %#v", receipt)
	}
	var bindingState string
	if err = db.QueryRow(`
SELECT json_extract(metadata_json, '$.execution_binding_state')
FROM session_goals
WHERE goal_id = ?`, goalID).Scan(&bindingState); err != nil {
		t.Fatal(err)
	}
	if bindingState != string(protocol.GoalExecutionBindingStateConfirmed) {
		t.Fatalf("Goal binding state = %q", bindingState)
	}
}
