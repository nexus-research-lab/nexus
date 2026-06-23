package automation

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func assertDeliveredAgentMessage(t *testing.T, workspacePath string, session protocol.Session, expectedText string, label string) {
	t.Helper()
	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatalf("读取%s消息失败: %v", label, err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望%s写入 1 条消息，实际 %d", label, len(messages))
	}
	if firstNonEmptyString(stringFromMessage(messages[0], "content")) != expectedText {
		t.Fatalf("%s正文不正确: %+v", label, messages[0])
	}
	summary, ok := messages[0]["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("%s应挂载 result_summary: %+v", label, messages[0])
	}
	if firstNonEmptyString(stringFromMessage(summary, "subtype")) != "success" {
		t.Fatalf("%s投递终态不正确: %+v", label, messages[0])
	}
}

func assertRunDeliveredTo(t *testing.T, service *Service, jobID string, expectedDeliveryTo string) protocol.CronRun {
	t.Helper()
	return assertRunDeliveredToContext(t, context.Background(), service, jobID, expectedDeliveryTo)
}

func assertRunDeliveredToContext(t *testing.T, ctx context.Context, service *Service, jobID string, expectedDeliveryTo string) protocol.CronRun {
	t.Helper()
	deliveredRuns, err := service.ListTaskRuns(ctx, jobID)
	if err != nil || len(deliveredRuns) == 0 {
		t.Fatalf("读取投递 run 失败: runs=%+v err=%v", deliveredRuns, err)
	}
	if deliveredRuns[0].DeliveryStatus != protocol.DeliveryStatusSucceeded {
		t.Fatalf("run delivery_status 未记录投递成功: %+v", deliveredRuns[0])
	}
	if deliveredRuns[0].DeliveryTo != expectedDeliveryTo {
		t.Fatalf("run delivery_to 应记录实际解析后的投递目标，实际 %q", deliveredRuns[0].DeliveryTo)
	}
	if deliveredRuns[0].DeliveryAttempts != 1 || deliveredRuns[0].DeliveredAt == nil {
		t.Fatalf("run 投递观测信息不完整: %+v", deliveredRuns[0])
	}
	return deliveredRuns[0]
}
