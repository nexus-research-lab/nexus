package roomrepo

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestPlanConversationDeletionPrefersMainFallback(t *testing.T) {
	conversations := []protocol.ConversationRecord{
		{ID: "topic-2", ConversationType: protocol.ConversationTypeTopic},
		{ID: "main", ConversationType: protocol.ConversationTypeMain},
		{ID: "topic-1", ConversationType: protocol.ConversationTypeTopic},
	}
	plan, err := planConversationDeletion(conversations, "topic-1")
	if err != nil {
		t.Fatalf("规划话题删除失败: %v", err)
	}
	if !plan.targetFound || plan.fallbackID != "main" {
		t.Fatalf("删除计划错误: %+v", plan)
	}
}

func TestPlanConversationDeletionRejectsMainConversation(t *testing.T) {
	conversations := []protocol.ConversationRecord{
		{ID: "main", ConversationType: protocol.ConversationTypeMain},
		{ID: "topic", ConversationType: protocol.ConversationTypeTopic},
	}
	_, err := planConversationDeletion(conversations, "main")
	if err == nil || !strings.Contains(err.Error(), "主对话") {
		t.Fatalf("删除主对话应返回明确错误，实际: %v", err)
	}
}

func TestPlanConversationDeletionKeepsMissingTargetDistinct(t *testing.T) {
	conversations := []protocol.ConversationRecord{
		{ID: "main", ConversationType: protocol.ConversationTypeMain},
		{ID: "topic", ConversationType: protocol.ConversationTypeTopic},
	}
	plan, err := planConversationDeletion(conversations, "missing")
	if err != nil {
		t.Fatalf("不存在的目标不应报错: %v", err)
	}
	if plan.targetFound {
		t.Fatalf("不存在的目标不应标记为找到: %+v", plan)
	}
}
