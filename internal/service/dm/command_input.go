// INPUT: DM 用户提交的原始 Slash 文本、附件组合与 host session 鉴权请求。
// OUTPUT: 可安全返回浏览器的原子命令输入校验，或无副作用的 host session 授权结果。
// POS: Slash 命令进入 runtime/host 前的 DM 业务边界。
package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type slashCommandAttachmentError struct{}

func (slashCommandAttachmentError) Error() string {
	return "slash commands do not accept attachments"
}

func (slashCommandAttachmentError) ClientMessage() string {
	return "Slash 指令必须作为独立文本发送，请先移除附件。"
}

// AuthorizeHostCommand 校验 Nexus host Slash 使用的 DM session 和 Agent owner，
// 不创建 session、不连接 runtime，避免目录展示或 host 派发产生隐式副作用。
func (s *Service) AuthorizeHostCommand(
	ctx context.Context,
	sessionKey string,
	requestedAgentID string,
) error {
	normalizedSessionKey, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return err
	}
	parsed := protocol.ParseSessionKey(normalizedSessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return ErrRoomSessionNotImplemented
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return errors.New("host Slash requires an agent session")
	}
	if strings.TrimSpace(parsed.ChatType) != "dm" {
		return errors.New("host Slash requires a DM session")
	}
	if requestedAgentID = strings.TrimSpace(requestedAgentID); requestedAgentID != "" &&
		requestedAgentID != parsed.AgentID {
		return errors.New("agent_id does not match session_key")
	}
	if s == nil || s.agents == nil {
		return errors.New("DM service is not configured")
	}
	agentID, err := s.resolveChatAgentID(ctx, parsed, requestedAgentID)
	if err != nil {
		return err
	}
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(agentValue.OwnerUserID) != authctx.OwnerUserID(ctx) {
		return errors.New("agent owner does not match request owner")
	}
	// host registry 当前由 WebSocket handler 驱动；外部渠道仍走各自的消息入口。
	if protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) != protocol.SessionChannelWebSocketSegment {
		return errors.New("host Slash requires a WebSocket session")
	}
	return nil
}
