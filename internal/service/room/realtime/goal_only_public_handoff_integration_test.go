// INPUT: Composer-equivalent user Goal, durable continuation receipt, Room public @ handoff and target terminal.
// OUTPUT: Goal-only source receipt settlement, terminal handback ledger and immediate next continuation.
// POS: Regression boundary for outer continuation RoundID versus per-slot AgentRoundID.
package realtime_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestRoomGoalOnlyPublicMentionHandbackImmediatelyContinues(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.GoalEnabled = true
	cfg.GoalAutoContinueEnabled = true
	cfg.GoalMaxContinuationsPerRun = 2
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("create Agent service: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	const ownerUserID = "owner-goal-only-public-handoff"
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: ownerUserID,
		Role:   authsvc.RoleOwner,
	})
	lead := createTestAgent(t, agentService, ctx, "Lead")
	researcher := createTestAgent(t, agentService, ctx, "Researcher")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{lead.AgentID, researcher.AgentID},
		Name:     "Goal-only public handoff regression",
		Title:    "Goal-only collaboration",
	})
	if err != nil {
		t.Fatalf("create Room: %v", err)
	}

	leadClient := newFakeRoomClient()
	researcherClient := newFakeRoomClient()
	var leadQueries atomic.Int32
	researcherResultSent := make(chan time.Time, 1)
	leadResumed := make(chan time.Time, 1)
	leadClient.onQuery = func(_ context.Context, _ string) error {
		switch leadQueries.Add(1) {
		case 1:
			go sendFakeAssistantResult(
				leadClient,
				"lead-goal-only-public-mention",
				"@Researcher 请给出一句链路核对结论。",
			)
		case 2:
			leadResumed <- time.Now()
			go sendFakeAssistantResult(
				leadClient,
				"lead-goal-only-after-handback",
				"已收到协作结果，继续完成目标检查。",
			)
		default:
			t.Errorf("unexpected extra Lead continuation: %d", leadQueries.Load())
		}
		return nil
	}
	researcherClient.onQuery = func(_ context.Context, _ string) error {
		sentAt := time.Now()
		go sendFakeAssistantResult(
			researcherClient,
			"researcher-goal-only-result",
			"Goal-only 公区协作链路已核对。",
		)
		researcherResultSent <- sentAt
		return nil
	}

	runtimeManager := runtimectx.NewManager()
	permission := permissionctx.NewContext()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{leadClient, researcherClient}},
	)
	goalRepository := goalstore.NewRepository(cfg, db)
	goalService := goalsvc.NewService(cfg, goalRepository)
	goalService.SetSessionOwnershipVerifier(testRoomGoalSessionOwnershipVerifier{
		agentNames: map[string]string{
			lead.AgentID:       lead.Name,
			researcher.AgentID: researcher.Name,
		},
	})
	service.SetGoalContextProvider(goalService)

	sessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := &realtimeTestSender{
		key:    "room-goal-only-public-handoff",
		events: make(chan protocol.EventMessage, 512),
	}
	permission.BindSession(sessionKey, sender)
	goalCommand, err := service.SetGoalFromCommand(ctx, protocol.GoalCommandRequest{
		SessionKey:      sessionKey,
		Objective:       "由 Lead 通过公区 @Researcher 完成一次 Goal-only 协作核对。",
		CommandContent:  "/goal 由 Lead 通过公区 @Researcher 完成一次 Goal-only 协作核对。",
		RoundID:         "round-composer-goal-only",
		UserMessageID:   "user-composer-goal-only",
		ClientRequestID: "request-composer-goal-only",
		ClientMessageID: "message-composer-goal-only",
		TargetAgentIDs:  []string{lead.AgentID},
	})
	if err != nil {
		t.Fatalf("set Room Goal from Composer command: %v", err)
	}
	if !goalCommand.UserMessageCommitted {
		t.Fatal("Composer Goal control record was not durable")
	}
	goal := &goalCommand.Goal
	if got := protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataExecutionMode); got != string(protocol.GoalExecutionModeGoalOnly) {
		t.Fatalf("Goal execution mode = %q, want goal_only", got)
	}
	if executionID := protocol.GoalReservedExecutionID(*goal); executionID != "" {
		t.Fatalf("Goal-only test unexpectedly reserved Execution %q", executionID)
	}

	firstPlan, err := goalService.PlanContinuationForSession(ctx, sessionKey, "")
	if err != nil || firstPlan == nil {
		t.Fatalf("plan initial Goal continuation = %+v, err=%v", firstPlan, err)
	}
	if err = service.DispatchGoalContinuation(ctx, *firstPlan); err != nil {
		t.Fatalf("dispatch initial Goal continuation: %v", err)
	}

	var researcherSentAt time.Time
	select {
	case researcherSentAt = <-researcherResultSent:
	case <-time.After(3 * time.Second):
		t.Fatal("public @Researcher did not start a target round")
	}
	var resumedAt time.Time
	select {
	case resumedAt = <-leadResumed:
	case <-time.After(3 * time.Second):
		t.Fatal("Goal handback did not immediately start the next continuation")
	}
	if delay := resumedAt.Sub(researcherSentAt); delay >= 3*time.Second {
		t.Fatalf("Goal handback continuation delay = %v, want immediate dispatch", delay)
	}

	var firstReceiptStatus string
	if err = db.QueryRowContext(
		ctx,
		`SELECT status FROM goal_continuation_plans WHERE round_id = ?`,
		firstPlan.RoundID,
	).Scan(&firstReceiptStatus); err != nil {
		t.Fatalf("read first continuation receipt: %v", err)
	}
	if firstReceiptStatus != string(protocol.GoalContinuationPlanStatusSettled) {
		t.Fatalf("first continuation receipt status = %q, want settled", firstReceiptStatus)
	}

	handoffs, err := workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath).ListRoot(
		ownerUserID,
		roomContext.Conversation.ID,
		firstPlan.RoundID,
	)
	if err != nil {
		t.Fatalf("read public handoff ledger: %v", err)
	}
	if len(handoffs) != 1 {
		t.Fatalf("public handoffs = %+v, want one Goal-attributed edge", handoffs)
	}
	handoff := handoffs[0]
	if handoff.GoalCollaborationBinding == nil ||
		handoff.GoalCollaborationBinding.GoalID != goal.ID ||
		handoff.GoalCollaborationBinding.ObjectiveRevision != goal.ObjectiveRevision() {
		t.Fatalf("Goal collaboration binding = %+v, want exact current Goal revision", handoff.GoalCollaborationBinding)
	}
	if handoff.Status != "finished" || !handoff.GoalHandbackSettled {
		t.Fatalf("public handoff terminal state = %+v, want finished plus settled handback", handoff)
	}
	if handoff.SourceAgentRoundID == "" || handoff.SourceAgentRoundID == firstPlan.RoundID {
		t.Fatalf(
			"source identities collapsed: receipt=%q agent_round=%q",
			firstPlan.RoundID,
			handoff.SourceAgentRoundID,
		)
	}
	if handoff.TargetRoundID == "" || handoff.TargetAgentRoundID == "" ||
		handoff.TargetRoundID == handoff.TargetAgentRoundID {
		t.Fatalf("target root/slot identities were not kept distinct: %+v", handoff)
	}
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage &&
			event.MessageID == "researcher-goal-only-result" &&
			event.Data["role"] == "assistant" && event.Data["is_complete"] == true &&
			protocol.NormalizePublicHandoffReply(event.Data["handoff_reply"]) != nil
	})
	var researcherReply protocol.Message
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage &&
			event.MessageID == "researcher-goal-only-result" &&
			event.Data["role"] == "assistant" && event.Data["is_complete"] == true &&
			protocol.NormalizePublicHandoffReply(event.Data["handoff_reply"]) != nil {
			researcherReply = protocol.Message(event.Data)
			break
		}
	}
	if researcherReply == nil {
		t.Fatalf("Goal-only researcher public reply missing from realtime events: %+v", events)
	}
	reply := protocol.NormalizePublicHandoffReply(researcherReply["handoff_reply"])
	if reply == nil || reply.HandoffID != handoff.HandoffID ||
		reply.SourceMessageID != "lead-goal-only-public-mention" ||
		reply.SourceAgentID != lead.AgentID {
		t.Fatalf("Goal-only public reply annotation = %+v, message=%+v", reply, researcherReply)
	}
	if researcherReply["agent_mentions"] != nil {
		t.Fatalf("no-@ Goal-only reply must not synthesize a reciprocal mention: %+v", researcherReply)
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(runtimeManager.GetRunningRoundIDs(sessionKey)) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if running := runtimeManager.GetRunningRoundIDs(sessionKey); len(running) != 0 {
		t.Fatalf("Room Goal-only rounds did not settle: %v", running)
	}
	if got := leadQueries.Load(); got != 2 {
		t.Fatalf("Lead queries = %d, want initial continuation plus one handback continuation", got)
	}
}
