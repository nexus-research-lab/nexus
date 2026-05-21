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

func TestEngineSkipsLowSignalTaskCompletion(t *testing.T) {
	workspace := t.TempDir()
	engine := NewEngine(workspace, DefaultOptions())
	scope := MemoryScope{
		Kind:       ScopeKindDMSession,
		UserID:     "owner",
		AgentID:    "nexus",
		SessionKey: "dm:web:nexus",
	}

	result, err := engine.CommitTurn(context.Background(), scope, CommittedTurn{
		UserText:      "你的 AGENTS.md 换行符被硬编码了，改成真正的换行符",
		AssistantText: "搞定了。原来的文件内容全部挤在一行里，已经用真实的换行符重写了整个文件，现在格式正常了。",
		SessionKey:    "dm:web:nexus",
		RoundID:       "round-low-signal",
		AgentID:       "nexus",
	})
	if err != nil {
		t.Fatalf("低信号对话判断失败: %v", err)
	}
	if !result.Skipped || result.Reason != "low_signal" {
		t.Fatalf("一次性任务完成摘要不应进入记忆: %+v", result)
	}
	items, err := engine.List(context.Background(), MemoryListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("读取记忆列表失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("低信号对话不应产生记忆条目: %+v", items)
	}
	if _, err = os.Stat(filepath.Join(workspace, "memory", "checkpoints.json")); !os.IsNotExist(err) {
		t.Fatalf("低信号对话不应推进 checkpoint: %v", err)
	}
}

func TestEngineCapturesDurableDecision(t *testing.T) {
	workspace := t.TempDir()
	engine := NewEngine(workspace, DefaultOptions())
	scope := MemoryScope{
		Kind:       ScopeKindDMSession,
		UserID:     "owner",
		AgentID:    "nexus",
		SessionKey: "dm:web:nexus",
	}

	result, err := engine.CommitTurn(context.Background(), scope, CommittedTurn{
		UserText:      "结论：Nexus 记忆需要分 User Memory 和 Agent Memory，两层不能混用。",
		AssistantText: "已按这个分层推进，用户偏好进入 User Memory，Agent 工作习惯进入 Agent Memory。",
		SessionKey:    "dm:web:nexus",
		RoundID:       "round-decision",
		AgentID:       "nexus",
	})
	if err != nil {
		t.Fatalf("稳定结论抽取失败: %v", err)
	}
	if !result.Processed || len(result.Items) != 1 {
		t.Fatalf("稳定结论应进入自动记忆: %+v", result)
	}
	if result.Items[0].Category != "decision" {
		t.Fatalf("稳定结论分类不正确: %+v", result.Items[0])
	}
	if result.Items[0].Kind != "LRN" {
		t.Fatalf("稳定结论应写成学习类记忆，而不是任务流水账: %+v", result.Items[0])
	}
	if strings.Contains(result.Items[0].Content, "结论：") || !strings.Contains(result.Items[0].Content, "User Memory 和 Agent Memory") {
		t.Fatalf("稳定结论内容应是可召回事实: %+v", result.Items[0])
	}
	for _, field := range result.Items[0].Fields {
		switch field.Key {
		case "做了什么", "结果", "反思", "经验":
			t.Fatalf("自动抽取不应再产生任务总结字段: %+v", result.Items[0].Fields)
		}
	}
}

func TestEngineCapturesIncidentAsErrorMemory(t *testing.T) {
	workspace := t.TempDir()
	engine := NewEngine(workspace, DefaultOptions())
	scope := MemoryScope{
		Kind:       ScopeKindDMSession,
		UserID:     "owner",
		AgentID:    "nexus",
		SessionKey: "dm:web:nexus",
	}

	result, err := engine.CommitTurn(context.Background(), scope, CommittedTurn{
		UserText:      "根因：Room 共享记忆串到 DM，是 scope 过滤遗漏。",
		AssistantText: "已修复 scopeCanAccessItem，DM 召回不会再读 Room 共享记忆。",
		SessionKey:    "dm:web:nexus",
		RoundID:       "round-incident",
		AgentID:       "nexus",
	})
	if err != nil {
		t.Fatalf("事故经验抽取失败: %v", err)
	}
	if !result.Processed || len(result.Items) != 1 {
		t.Fatalf("事故经验应进入自动记忆: %+v", result)
	}
	if result.Items[0].Kind != "ERR" || result.Items[0].Category != "incident" {
		t.Fatalf("事故经验应写成错误类记忆: %+v", result.Items[0])
	}
	if strings.Contains(result.Items[0].Content, "根因：") || !strings.Contains(result.Items[0].Content, "scope 过滤遗漏") {
		t.Fatalf("事故经验内容应是可复用根因: %+v", result.Items[0])
	}
	items, err := engine.List(context.Background(), MemoryListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("重新读取事故记忆失败: %v", err)
	}
	if len(items) != 1 || items[0].Category != "incident" {
		t.Fatalf("事故记忆重新解析后应保留 category: %+v", items)
	}
}

func TestEngineSkipsReadmeImageTask(t *testing.T) {
	workspace := t.TempDir()
	engine := NewEngine(workspace, DefaultOptions())
	scope := MemoryScope{
		Kind:       ScopeKindDMSession,
		UserID:     "owner",
		AgentID:    "nexus",
		SessionKey: "dm:web:nexus",
	}

	result, err := engine.CommitTurn(context.Background(), scope, CommittedTurn{
		UserText:      "根据内容画一个插图，简笔风格的，适合放在readme里面",
		AssistantText: "已生成：`output/imagegen/nexus-readme-illustration.png`",
		SessionKey:    "dm:web:nexus",
		RoundID:       "round-readme-image",
		AgentID:       "nexus",
	})
	if err != nil {
		t.Fatalf("README 插图任务判断失败: %v", err)
	}
	if !result.Skipped || result.Reason != "low_signal" {
		t.Fatalf("一次性图片生成任务不应进入记忆: %+v", result)
	}
}

func TestEngineCleanupRemovesOrphanSessionAndCheckpoint(t *testing.T) {
	workspace := t.TempDir()
	memoryDir := filepath.Join(workspace, "memory")
	sessionsDir := filepath.Join(memoryDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("创建 session 目录失败: %v", err)
	}
	sessionPath := filepath.Join(sessionsDir, "dm-web-nexus.md")
	if err := os.WriteFile(sessionPath, []byte("## 2026-05-21 10:00\n\n- Entry: REF-20260521-100000-stale\n- User: 一次性任务\n"), 0o644); err != nil {
		t.Fatalf("写入孤立 session 摘要失败: %v", err)
	}
	checkpoints := memoryCheckpoints{Scopes: map[string]memoryScopeCheckpoint{
		"dm_session:nexus:dm:web:nexus": {
			TurnCount:   1,
			LastRoundID: "round-stale",
			RoundIDs:    []string{"round-stale"},
		},
	}}
	repository := NewRepository(workspace)
	if err := repository.WriteCheckpoints(checkpoints); err != nil {
		t.Fatalf("写入 checkpoint 失败: %v", err)
	}

	result, err := NewEngine(workspace, DefaultOptions()).Cleanup(context.Background())
	if err != nil {
		t.Fatalf("清理孤立记忆失败: %v", err)
	}
	if result.RemovedSessionFiles != 1 || result.RemovedCheckpoints != 1 {
		t.Fatalf("清理结果不正确: %+v", result)
	}
	if _, err = os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("孤立 session 摘要应被删除: %v", err)
	}
	if _, err = os.Stat(filepath.Join(memoryDir, "checkpoints.json")); !os.IsNotExist(err) {
		t.Fatalf("孤立 checkpoint 应被删除: %v", err)
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

func TestExtractMessageTextSupportsTypedContentBlocks(t *testing.T) {
	message := map[string]any{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "第一段"},
			{"type": "text", "text": "第二段"},
		},
	}

	text := ExtractMessageText(message)
	if text != "第一段\n第二段" {
		t.Fatalf("typed content blocks 提取失败: %q", text)
	}
}
