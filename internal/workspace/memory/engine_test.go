package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEngineCommitRecallAndCheckpoint(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "MEMORY.md"), []byte("# MEMORY.md\n\n"), 0o644); err != nil {
		t.Fatalf("写入 MEMORY.md 失败: %v", err)
	}
	engine := NewEngine(workspace, DefaultOptions())
	scope := MemoryScope{
		Kind:       ScopeKindDMSession,
		UserID:     "owner",
		AgentID:    "nexus",
		SessionKey: "dm:web:nexus",
	}

	captured, err := engine.CommitTurn(context.Background(), scope, CommittedTurn{
		UserText:      "以后默认注释都使用中文，复杂逻辑必须写清楚原因",
		AssistantText: "已记录，后续注释保持中文并解释复杂逻辑。",
		SessionKey:    "dm:web:nexus",
		RoundID:       "round-1",
		AgentID:       "nexus",
	})
	if err != nil {
		t.Fatalf("提交自动记忆失败: %v", err)
	}
	if !captured.Processed || len(captured.Items) != 1 {
		t.Fatalf("预期首轮高影响记忆被捕获: %+v", captured)
	}
	if captured.Items[0].Status != "candidate" {
		t.Fatalf("高影响规则必须先进入候选区: %+v", captured.Items[0])
	}

	duplicate, err := engine.CommitTurn(context.Background(), scope, CommittedTurn{
		UserText:      "以后默认注释都使用中文",
		AssistantText: "收到。",
		SessionKey:    "dm:web:nexus",
		RoundID:       "round-1",
		AgentID:       "nexus",
	})
	if err != nil {
		t.Fatalf("重复 checkpoint 判断失败: %v", err)
	}
	if !duplicate.Skipped || duplicate.Reason != "duplicate_round" {
		t.Fatalf("重复 round 不应再次抽取: %+v", duplicate)
	}

	injection, err := engine.BeforeRecall(context.Background(), scope, RecallRequest{
		Query:      "中文 注释",
		MaxResults: 3,
	})
	if err != nil {
		t.Fatalf("召回记忆失败: %v", err)
	}
	if len(injection.Items) != 1 || !strings.Contains(injection.DynamicUserContext, "<relevant-memories>") {
		t.Fatalf("召回结果不正确: %+v", injection)
	}

	checkpointPath := filepath.Join(workspace, "memory", "checkpoints.json")
	if _, err := os.Stat(checkpointPath); err != nil {
		t.Fatalf("checkpoint 未落盘: %v", err)
	}
}

func TestEngineScopeRankingAndDelete(t *testing.T) {
	workspace := t.TempDir()
	engine := NewEngine(workspace, DefaultOptions())
	dmScope := MemoryScope{
		Kind:       ScopeKindDMSession,
		AgentID:    "nexus",
		SessionKey: "dm:web:nexus",
	}
	roomScope := MemoryScope{
		Kind:           ScopeKindRoomShared,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	agentScope := MemoryScope{
		Kind:    ScopeKindAgent,
		AgentID: "nexus",
	}
	dmItem, err := engine.Add(context.Background(), dmScope, MemoryWriteInput{
		Title:   "中文注释偏好",
		Content: "用户偏好所有复杂逻辑注释使用中文。",
		Status:  "candidate",
	})
	if err != nil {
		t.Fatalf("新增 DM 记忆失败: %v", err)
	}
	if _, err = engine.Add(context.Background(), roomScope, MemoryWriteInput{
		Title:   "Room 中文注释偏好",
		Content: "Room 共享讨论里也提到中文注释。",
		Status:  "candidate",
	}); err != nil {
		t.Fatalf("新增 Room 记忆失败: %v", err)
	}
	if _, err = engine.Add(context.Background(), agentScope, MemoryWriteInput{
		Title:   "Agent 中文注释偏好",
		Content: "Agent 级别也保留中文注释偏好。",
		Status:  "candidate",
	}); err != nil {
		t.Fatalf("新增 Agent 记忆失败: %v", err)
	}

	items, err := engine.Search(context.Background(), dmScope, RecallRequest{
		Query:      "中文 注释",
		MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("检索记忆失败: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("预期命中 DM 与 Agent 记忆: %+v", items)
	}
	if items[0].EntryID != dmItem.EntryID {
		t.Fatalf("同 scope 记忆应排在前面: %+v", items)
	}
	for _, item := range items {
		if strings.HasPrefix(item.Scope, string(ScopeKindRoomShared)+":") {
			t.Fatalf("DM 召回不应串入 Room 共享记忆: %+v", items)
		}
	}

	if err = engine.Delete(context.Background(), dmItem.EntryID); err != nil {
		t.Fatalf("删除记忆失败: %v", err)
	}
	items, err = engine.Search(context.Background(), dmScope, RecallRequest{
		Query:      "中文 注释",
		MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("删除后检索失败: %v", err)
	}
	for _, item := range items {
		if item.EntryID == dmItem.EntryID {
			t.Fatalf("删除后的记忆不应被召回: %+v", items)
		}
	}
}

func TestEngineIgnoreRemovesFromRecall(t *testing.T) {
	workspace := t.TempDir()
	engine := NewEngine(workspace, DefaultOptions())
	scope := MemoryScope{Kind: ScopeKindAgent, AgentID: "nexus"}
	item, err := engine.Add(context.Background(), scope, MemoryWriteInput{
		Title:   "偏好候选",
		Content: "以后默认使用中文回复。",
		Status:  "candidate",
	})
	if err != nil {
		t.Fatalf("新增记忆失败: %v", err)
	}
	if _, err = engine.Ignore(context.Background(), item.EntryID, "不适用于当前场景"); err != nil {
		t.Fatalf("忽略记忆失败: %v", err)
	}
	items, err := engine.Search(context.Background(), scope, RecallRequest{
		Query:      "中文 回复",
		MaxResults: 5,
	})
	if err != nil {
		t.Fatalf("忽略后检索失败: %v", err)
	}
	for _, recalled := range items {
		if recalled.EntryID == item.EntryID {
			t.Fatalf("忽略后的记忆不应被召回: %+v", items)
		}
	}
}

func TestMemorySchedulerCadence(t *testing.T) {
	workspace := t.TempDir()
	scheduler := NewMemoryScheduler(NewRepository(workspace))
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.Local)

	first, err := scheduler.Advance("dm:nexus", "round-1", now, false)
	if err != nil {
		t.Fatalf("首轮调度失败: %v", err)
	}
	if !first.ShouldCapture || first.Reason != "captured" {
		t.Fatalf("首轮应立即抽取: %+v", first)
	}

	duplicate, err := scheduler.Advance("dm:nexus", "round-1", now.Add(time.Minute), true)
	if err != nil {
		t.Fatalf("重复轮次调度失败: %v", err)
	}
	if duplicate.ShouldCapture || duplicate.Reason != "duplicate_round" {
		t.Fatalf("重复 round 不应抽取: %+v", duplicate)
	}

	wait, err := scheduler.Advance("dm:nexus", "round-2", now.Add(2*time.Minute), false)
	if err != nil {
		t.Fatalf("普通轮次调度失败: %v", err)
	}
	if wait.ShouldCapture || wait.Reason != "scheduler_wait" {
		t.Fatalf("未到 5 轮或 10 分钟不应抽取: %+v", wait)
	}

	idle, err := scheduler.Advance("dm:nexus", "round-3", now.Add(12*time.Minute), false)
	if err != nil {
		t.Fatalf("空闲触发调度失败: %v", err)
	}
	if !idle.ShouldCapture {
		t.Fatalf("空闲 10 分钟后应抽取: %+v", idle)
	}
}
