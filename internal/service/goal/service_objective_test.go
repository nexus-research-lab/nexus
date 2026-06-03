package goal

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceCreateGoalRewritesObjectiveByDefault(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetObjectiveRewriter(fakeObjectiveRewriter{rewritten: "完成 Goal 对齐并验证关键路径"})

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "把 goal 分支修到和 Codex 差不多",
		OwnerUserID: "owner-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Objective != "完成 Goal 对齐并验证关键路径" {
		t.Fatalf("objective = %q", created.Objective)
	}
	if created.Metadata["source_objective"] != "把 goal 分支修到和 Codex 差不多" ||
		created.Metadata["objective_normalized"] != true {
		t.Fatalf("metadata = %#v, want source objective and normalized marker", created.Metadata)
	}
	if len(repo.events) != 1 || repo.events[0].Payload["objective"] != created.Objective {
		t.Fatalf("events = %#v, want created event with rewritten objective", repo.events)
	}
}

func TestServiceCreateGoalFallsBackWhenRewriteFails(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetObjectiveRewriter(fakeObjectiveRewriter{})

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "保留原始目标",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Objective != "保留原始目标" {
		t.Fatalf("objective = %q, want fallback original", created.Objective)
	}
	if len(created.Metadata) != 0 {
		t.Fatalf("metadata = %#v, want no rewrite metadata", created.Metadata)
	}
}

func TestServiceCreateGoalKeepsModelObjective(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetObjectiveRewriter(fakeObjectiveRewriter{rewritten: "不应改写模型目标"})

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "模型已经整理后的目标",
		CreatedBy:  "model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Objective != "模型已经整理后的目标" {
		t.Fatalf("objective = %q, want model objective unchanged", created.Objective)
	}
	if len(created.Metadata) != 0 {
		t.Fatalf("metadata = %#v, want no rewrite metadata", created.Metadata)
	}
}

func TestServiceUpdateGoalRewritesObjectiveByDefault(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "初始目标",
	})
	if err != nil {
		t.Fatal(err)
	}
	service.SetObjectiveRewriter(fakeObjectiveRewriter{rewritten: "整理后的更新目标"})

	updatedObjective := "把目标更新成一段比较长的描述"
	updated, err := service.Update(context.Background(), created.ID, protocol.UpdateGoalRequest{
		Objective:   &updatedObjective,
		OwnerUserID: "owner-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Objective != "整理后的更新目标" {
		t.Fatalf("objective = %q", updated.Objective)
	}
	if len(repo.events) != 2 ||
		repo.events[1].Payload["source_objective"] != updatedObjective ||
		repo.events[1].Payload["objective_normalized"] != true {
		t.Fatalf("events = %#v, want update event with rewrite markers", repo.events)
	}
}

func TestServiceThreadGoalSetRewritesObjectiveByDefault(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetObjectiveRewriter(fakeObjectiveRewriter{rewritten: "整理后的 app-server 目标"})

	objective := "app-server 设定的一段比较长的目标"
	created, err := service.SetFromThreadGoalParams(context.Background(), protocol.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:chat",
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Objective != "整理后的 app-server 目标" {
		t.Fatalf("objective = %q", created.Objective)
	}
	if created.Metadata["created_via"] != "thread_goal_set" ||
		created.Metadata["source_objective"] != objective ||
		created.Metadata["objective_normalized"] != true {
		t.Fatalf("metadata = %#v, want app-server rewrite metadata", created.Metadata)
	}
}

func TestServiceObjectiveRewriteReceivesSessionKey(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	rewriter := &recordingObjectiveRewriter{
		outputs: []string{
			"整理后的创建目标",
			"整理后的更新目标",
			"整理后的 app-server 创建目标",
			"整理后的 app-server 更新目标",
		},
	}
	service.SetObjectiveRewriter(rewriter)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "创建目标草稿",
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedObjective := "更新目标草稿"
	if _, err = service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &updatedObjective}); err != nil {
		t.Fatal(err)
	}
	roomObjective := "app-server 创建草稿"
	appCreated, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  "room:group:conversation-1",
		Objective: &roomObjective,
	})
	if err != nil {
		t.Fatal(err)
	}
	roomUpdateObjective := "app-server 更新草稿"
	if _, err = service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  appCreated.SessionKey,
		Objective: &roomUpdateObjective,
	}); err != nil {
		t.Fatal(err)
	}

	wantSessionKeys := []string{
		"agent:nexus:ws:dm:chat",
		"agent:nexus:ws:dm:chat",
		"room:group:conversation-1",
		"room:group:conversation-1",
	}
	if len(rewriter.calls) != len(wantSessionKeys) {
		t.Fatalf("rewrite calls = %#v, want %d calls", rewriter.calls, len(wantSessionKeys))
	}
	for index, want := range wantSessionKeys {
		if rewriter.calls[index].sessionKey != want {
			t.Fatalf("call %d session_key = %q, want %q; calls=%#v", index, rewriter.calls[index].sessionKey, want, rewriter.calls)
		}
	}
}

type fakeObjectiveRewriter struct {
	rewritten string
}

func (f fakeObjectiveRewriter) RewriteGoalObjective(context.Context, string, string, string) (string, error) {
	return f.rewritten, nil
}

type recordingObjectiveRewriter struct {
	outputs []string
	calls   []recordingObjectiveRewriteCall
}

type recordingObjectiveRewriteCall struct {
	ownerUserID string
	sessionKey  string
	objective   string
}

func (r *recordingObjectiveRewriter) RewriteGoalObjective(_ context.Context, ownerUserID string, sessionKey string, objective string) (string, error) {
	r.calls = append(r.calls, recordingObjectiveRewriteCall{
		ownerUserID: ownerUserID,
		sessionKey:  sessionKey,
		objective:   objective,
	})
	if len(r.outputs) == 0 {
		return objective, nil
	}
	output := r.outputs[0]
	r.outputs = r.outputs[1:]
	return output, nil
}
