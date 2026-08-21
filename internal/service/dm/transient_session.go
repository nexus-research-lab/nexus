// INPUT: 宿主签发的 Agent、隐藏 DM session key 与单一 WorkGraph 用途。
// OUTPUT: 不继承任何源 transcript、从普通目录隐藏的编辑或后台 Session。
// POS: WorkGraph 编辑/保存与普通 Agent DM、来源会话 transcript 之间的硬隔离边界。
package dm

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// TransientSessionRequest 只由宿主组合层构造；HTTP/WS 不能选择内部身份或用途。
type TransientSessionRequest struct {
	AgentID               string
	TargetSessionKey      string
	Purpose               string
	Title                 string
	DisplayAfterUnixMilli int64
}

// CreateTransientSession 创建不 fork、也不进入普通目录的宿主专用 DM Session。
func (s *Service) CreateTransientSession(
	ctx context.Context,
	request TransientSessionRequest,
) (*protocol.Session, error) {
	if s == nil {
		return nil, errors.New("DM service is unavailable")
	}
	agentID := strings.TrimSpace(request.AgentID)
	targetSessionKey := strings.TrimSpace(request.TargetSessionKey)
	purpose := strings.TrimSpace(request.Purpose)
	parsed := protocol.ParseSessionKey(targetSessionKey)
	allowedChannel := protocol.SessionChannelInternalSegment
	if purpose == protocol.SessionPurposeWorkGraphEditor {
		allowedChannel = protocol.SessionChannelWebSocketSegment
	}
	if agentID == "" || purpose == "" ||
		!parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent ||
		strings.TrimSpace(parsed.AgentID) != agentID ||
		protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) != allowedChannel ||
		strings.TrimSpace(parsed.ChatType) != protocol.RoomTypeDM {
		return nil, errors.New("transient internal Session identity is invalid")
	}
	agentValue, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(agentValue.OwnerUserID) != authctx.OwnerUserID(ctx) {
		return nil, errors.New("transient internal Session owner does not match Agent owner")
	}
	targetSession, err := s.ensureSession(ctx, agentValue, parsed, targetSessionKey)
	if err != nil {
		return nil, err
	}
	if existingPurpose := protocol.SessionPurpose(targetSession); existingPurpose != "" && existingPurpose != purpose {
		return nil, errors.New("transient internal Session purpose already differs")
	}
	if targetSession.Options == nil {
		targetSession.Options = map[string]any{}
	}
	targetSession.Title = strings.TrimSpace(request.Title)
	if targetSession.Title == "" {
		targetSession.Title = "Internal task"
	}
	targetSession.Options[protocol.OptionSessionHiddenFromDirectory] = true
	targetSession.Options[protocol.OptionSessionPurpose] = purpose
	if request.DisplayAfterUnixMilli > 0 {
		targetSession.Options[protocol.OptionSessionDisplayAfterUnixMilli] = request.DisplayAfterUnixMilli
	}
	created, err := s.files.ForOwner(agentValue.OwnerUserID).UpsertSession(
		agentValue.WorkspacePath,
		targetSession,
	)
	if err != nil {
		return nil, err
	}
	if created != nil {
		targetSession = *created
	}
	return &targetSession, nil
}
