package room_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
)

type firstBlockingRoomQuotaChecker struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
}

type blockingInputQueueBroadcaster struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (b *blockingInputQueueBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	if event.EventType == protocol.EventTypeInputQueue {
		b.once.Do(func() {
			close(b.entered)
			<-b.release
		})
	}
	return nil
}

func (q *firstBlockingRoomQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	q.mu.Lock()
	q.calls++
	first := q.calls == 1
	q.mu.Unlock()
	if first {
		close(q.entered)
		<-q.release
	}
	return nil
}

func (q *firstBlockingRoomQuotaChecker) callCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.calls
}

func TestRoomExplicitInputWinsGoalContinuationDispatchRace(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.GoalEnabled = true
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-room-goal-race",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "Goal 竞态助手")
	roomContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatal(err)
	}

	client := newFakeRoomClient()
	queuedClient := newFakeRoomClient()
	runtimeManager := runtimectx.NewManager()
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{client, queuedClient}},
	)
	broadcaster := &blockingInputQueueBroadcaster{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	service.SetRoomBroadcaster(broadcaster)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	service.SetGoalContextProvider(goalService)
	quota := &firstBlockingRoomQuotaChecker{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	service.SetQuotaChecker(quota)

	sessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	goal, err := goalService.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  sessionKey,
		Objective:   "在用户输入后继续",
		OwnerUserID: "owner-room-goal-race",
	})
	if err != nil {
		t.Fatal(err)
	}

	explicitResult := make(chan error, 1)
	go func() {
		explicitResult <- service.HandleChat(ctx, roomsvc.ChatRequest{
			SessionKey:     sessionKey,
			RoomID:         roomContext.Room.ID,
			ConversationID: roomContext.Conversation.ID,
			Content:        "用户现在的指令",
			RoundID:        "round-explicit",
		})
	}()
	<-quota.entered

	continuationResult := make(chan error, 1)
	go func() {
		continuationResult <- service.DispatchGoalContinuation(ctx, protocol.GoalContinuation{
			Goal:           *goal,
			RoundID:        "round-goal-continuation",
			Prompt:         "continue goal",
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "goal_continuation",
		})
	}()
	close(quota.release)

	if err = <-explicitResult; err != nil {
		t.Fatalf("explicit HandleChat failed: %v", err)
	}
	if err = <-continuationResult; err != nil {
		t.Fatalf("Goal continuation dispatch failed: %v", err)
	}
	if got := quota.callCount(); got != 1 {
		t.Fatalf("quota checks = %d, want only the explicit input to reach runtime startup", got)
	}
	if running := runtimeManager.GetRunningRoundIDs(sessionKey); !slices.Contains(running, "round-explicit") || slices.Contains(running, "round-goal-continuation") {
		t.Fatalf("running rounds = %v, want explicit input without Goal continuation", running)
	}

	cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	if _, err = runtimeManager.InterruptSession(cleanupCtx, sessionKey, "test cleanup"); err != nil {
		cancel()
		t.Fatalf("cleanup Room round failed: %v", err)
	}
	cancel()

	enqueueResult := make(chan error, 1)
	go func() {
		enqueueResult <- service.HandleInputQueue(ctx, roomsvc.InputQueueRequest{
			SessionKey:     sessionKey,
			RoomID:         roomContext.Room.ID,
			ConversationID: roomContext.Conversation.ID,
			Action:         "enqueue",
			Content:        "排队的用户指令",
		})
	}()
	<-broadcaster.entered

	queuedContinuationResult := make(chan error, 1)
	go func() {
		queuedContinuationResult <- service.DispatchGoalContinuation(ctx, protocol.GoalContinuation{
			Goal:           *goal,
			RoundID:        "round-goal-after-enqueue",
			Prompt:         "continue goal",
			HiddenFromUser: true,
			Synthetic:      true,
			Purpose:        "goal_continuation",
		})
	}()
	close(broadcaster.release)

	if err = <-enqueueResult; err != nil {
		t.Fatalf("enqueue user input failed: %v", err)
	}
	if err = <-queuedContinuationResult; err != nil {
		t.Fatalf("Goal continuation after enqueue failed: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for quota.callCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := quota.callCount(); got != 2 {
		t.Fatalf("quota checks = %d, want one direct and one queued user input", got)
	}
	deadline = time.Now().Add(time.Second)
	for len(runtimeManager.GetRunningRoundIDs(sessionKey)) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	running := runtimeManager.GetRunningRoundIDs(sessionKey)
	if len(running) == 0 || slices.Contains(running, "round-goal-after-enqueue") {
		t.Fatalf("running rounds = %v, want queued user input without Goal continuation", running)
	}

	cleanupCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	if _, err = runtimeManager.InterruptSession(cleanupCtx, sessionKey, "test cleanup"); err != nil {
		cancel()
		t.Fatalf("cleanup queued Room round failed: %v", err)
	}
	cancel()
}
