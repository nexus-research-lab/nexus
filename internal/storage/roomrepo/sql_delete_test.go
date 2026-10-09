package roomrepo

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestPlanConversationDeletionRejectsFinalConversation(t *testing.T) {
	conversations := []protocol.ConversationRecord{
		{ID: "main", ConversationType: protocol.ConversationTypeMain},
	}
	_, err := planConversationDeletion(conversations, "main")
	if err == nil || !strings.Contains(err.Error(), "至少保留") {
		t.Fatalf("删除最后一个对话应返回明确错误，实际: %v", err)
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
