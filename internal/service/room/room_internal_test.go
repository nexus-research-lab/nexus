// room_internal_test.go 覆盖 Room 内部 Agent 解析与目录监听测试。
package room

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func TestResolveAgentWorkspacePathRejectsRoomOwnerMismatch(t *testing.T) {
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "user-a",
	})

	_, err := (&Service{}).resolveAgentWorkspacePath(ctx, "user-b", "agent-a")
	if err == nil || !strings.Contains(err.Error(), "Room owner 与调用上下文不一致") {
		t.Fatalf("跨 owner 的 Room 清理应在查询 Agent 前拒绝: %v", err)
	}
}
