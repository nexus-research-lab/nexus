// INPUT: 宿主注入的外部输入回复与权限适配器。
// OUTPUT: 仅按持久 root 和成员身份关联的私域完成回复及权限处理。
// POS: Room 保持调度权，外部 transport 不参与公区事件或成员选择。
package realtime

import (
	"context"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
)

// SetExternalInputHooks 在服务启动前连接外部 transport，不改变 Room 调度。
func (s *Service) SetExternalInputHooks(
	reply func(context.Context, string, string, string, protocol.Message) error,
	permission func(context.Context, string, string, string) (sdkpermission.Handler, error),
	prompt func(context.Context, string, string, string) (string, error),
) {
	s.externalReply = reply
	s.externalPermission = permission
	s.externalPrompt = prompt
}

// deliverExternalCompletion 只由整轮成功终态调用；单条 assistant 完成不代表最终答复。
func (e *slotExecution) deliverExternalCompletion(result exec.RoundExecutionResult, value protocol.Message) error {
	if !result.CompletedByAssistant || roomSlotTerminalStatus(result) != "finished" || e.slot.shouldSuppressOutput() || e.service.externalReply == nil || roomdomain.IsNoReplyAssistantMessage(value) {
		return nil
	}
	return e.service.externalReply(e.ctx, roomRootRoundID(e.round), e.slot.AgentID, e.slot.RuntimeSessionKey, roomdomain.StripNoReplyMarker(value))
}
