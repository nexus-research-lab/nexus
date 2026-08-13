// INPUT: DM 用户提交的原始 Slash 文本、附件组合与 session/host 鉴权请求。
// OUTPUT: 可安全返回浏览器的原子命令输入校验，或无副作用的 DM/host 授权结果。
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

// AuthorizeDMSessionAccess 校验浏览器订阅的 DM session 和 Agent owner。
// 外部 IM session 也允许其 owner 查看，但不会因此获得 Web host Slash 能力。
func (s *Service) AuthorizeDMSessionAccess(
	ctx context.Context,
	sessionKey string,
	requestedAgentID string,
) error {
	_, err := s.authorizeDMSessionAccess(ctx, sessionKey, requestedAgentID)
	return err
}

// AuthorizeHostCommand 校验 Nexus host Slash 使用的 WebSocket DM session。
// 不创建 session、不连接 runtime，避免目录展示或 host 派发产生隐式副作用。
func (s *Service) AuthorizeHostCommand(
	ctx context.Context,
	sessionKey string,
	requestedAgentID string,
) error {
	parsed, err := s.authorizeDMSessionAccess(ctx, sessionKey, requestedAgentID)
	if err != nil {
		return err
	}
	// host registry 当前由 WebSocket handler 驱动；外部渠道仍走各自的消息入口。
	if protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) != protocol.SessionChannelWebSocketSegment {
		return errors.New("host Slash requires a WebSocket session")
	}
	return nil
}

func (s *Service) authorizeDMSessionAccess(
	ctx context.Context,
	sessionKey string,
	requestedAgentID string,
) (protocol.SessionKey, error) {
	normalizedSessionKey, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return protocol.SessionKey{}, err
	}
	parsed := protocol.ParseSessionKey(normalizedSessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return protocol.SessionKey{}, ErrRoomSessionNotImplemented
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return protocol.SessionKey{}, errors.New("DM access requires an agent session")
	}
	if strings.TrimSpace(parsed.ChatType) != "dm" {
		return protocol.SessionKey{}, errors.New("DM access requires a DM session")
	}
	if requestedAgentID = strings.TrimSpace(requestedAgentID); requestedAgentID != "" &&
		requestedAgentID != parsed.AgentID {
		return protocol.SessionKey{}, errors.New("agent_id does not match session_key")
	}
	if s == nil || s.agents == nil {
		return protocol.SessionKey{}, errors.New("DM service is not configured")
	}
	agentID, err := s.resolveChatAgentID(ctx, parsed, requestedAgentID)
	if err != nil {
		return protocol.SessionKey{}, err
	}
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return protocol.SessionKey{}, err
	}
	if strings.TrimSpace(agentValue.OwnerUserID) != authctx.OwnerUserID(ctx) {
		return protocol.SessionKey{}, errors.New("agent owner does not match request owner")
	}
	return parsed, nil
}
