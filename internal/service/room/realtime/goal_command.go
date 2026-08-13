// INPUT: 已授权 `/goal` 或 UI set_goal、Room member/lead 事实与 Goal service。
// OUTPUT: 服务端验证 lead 后的 Goal、durable 公区控制记录与 draft 消费。
// POS: Room Goal command 的 conversation 级串行业务边界；不创建普通模型 slot。
package realtime

import (
	"context"
	"errors"
	"strconv"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

type goalCommandProvider interface {
	Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error)
}

// SetGoalFromCommand 设置共享 Goal，并把用户控制记录写入 Room 公共历史。
func (s *Service) SetGoalFromCommand(
	ctx context.Context,
	request protocol.GoalCommandRequest,
) (protocol.GoalCommandResult, error) {
	sessionKey, conversationID, err := s.validateChatRequest(ChatRequest{
		SessionKey:                  request.SessionKey,
		Content:                     request.CommandContent,
		ClientRequestID:             request.ClientRequestID,
		ClientMessageID:             request.ClientMessageID,
		RoundID:                     request.RoundID,
		UserMessageID:               request.UserMessageID,
		TargetAgentIDs:              request.TargetAgentIDs,
		DeliveryPolicy:              protocol.ChatDeliveryPolicyAuto,
		TrustedConfigurationContext: true,
	})
	if err != nil {
		return protocol.GoalCommandResult{}, err
	}
	lease := s.lockRoomDispatch(sessionKey, conversationID)
	defer lease.Unlock()
	execution, err := s.prepareRoomChat(ctx, ChatRequest{
		SessionKey:                  sessionKey,
		Content:                     request.CommandContent,
		ClientRequestID:             request.ClientRequestID,
		ClientMessageID:             request.ClientMessageID,
		RoundID:                     request.RoundID,
		UserMessageID:               request.UserMessageID,
		TargetAgentIDs:              request.TargetAgentIDs,
		DeliveryPolicy:              protocol.ChatDeliveryPolicyAuto,
		TrustedConfigurationContext: true,
	})
	if err != nil {
		return protocol.GoalCommandResult{}, err
	}
	if len(execution.targetAgentIDs) != 1 {
		return protocol.GoalCommandResult{}, errors.New("Room Goal requires exactly one lead Agent")
	}
	provider, ok := s.goals.(goalCommandProvider)
	if !ok || provider == nil {
		return protocol.GoalCommandResult{}, errors.New("Goal service is unavailable")
	}
	replaceExisting := true
	if request.Options.ReplaceExisting != nil {
		replaceExisting = *request.Options.ReplaceExisting
	}
	leadAgentID := execution.targetAgentIDs[0]
	item, err := provider.Create(
		goalsvc.WithActiveGoalContinuationSuppressed(ctx),
		protocol.CreateGoalRequest{
			SessionKey:      execution.sessionKey,
			Objective:       strings.TrimSpace(request.Objective),
			TokenBudget:     request.Options.TokenBudget,
			ReplaceExisting: replaceExisting,
			CreatedBy:       "user",
			RoundID:         execution.request.RoundID,
			OwnerUserID:     authctx.OwnerUserID(ctx),
			RoomLeadAgentID: leadAgentID,
			Metadata:        request.Options.Metadata,
		},
	)
	if err != nil {
		return protocol.GoalCommandResult{}, err
	}
	if item == nil || strings.TrimSpace(item.ID) == "" {
		return protocol.GoalCommandResult{}, errors.New("Goal service returned an invalid Goal")
	}
	if goalsvc.RoomLeadAgentID(*item) != leadAgentID {
		return protocol.GoalCommandResult{}, errors.New("Goal service returned inconsistent Room command state")
	}
	committed := execution.persistGoalCommandRecord(*item)
	return protocol.GoalCommandResult{
		Goal:                 *item,
		UserMessageCommitted: committed,
	}, nil
}

func (e *roomChatExecution) persistGoalCommandRecord(item protocol.Goal) bool {
	if e == nil || e.service == nil {
		return false
	}
	e.userMessage["metadata"] = map[string]any{
		"subtype":                 "goal_set",
		"goal_id":                 item.ID,
		"goal_objective_revision": strconv.FormatInt(item.ObjectiveRevision(), 10),
	}
	e.userMessage["control_only"] = true
	if err := e.service.persistSharedInlineMessage(
		e.contextValue.Room.OwnerUserID,
		e.conversationID,
		e.userMessage,
	); err != nil {
		e.service.loggerFor(e.ctx).Error("Goal 已设置，但 Room 控制记录持久化失败",
			"session_key", e.sessionKey,
			"goal_id", item.ID,
			"round_id", e.request.RoundID,
			"err", err,
		)
		return false
	}
	if err := e.service.markConversationStarted(
		e.ctx,
		e.conversationID,
		roomMessageActivityTime(e.userMessage),
	); err != nil {
		e.service.loggerFor(e.ctx).Warn("Goal 控制记录已持久化，但 conversation draft 状态更新失败",
			"session_key", e.sessionKey,
			"goal_id", item.ID,
			"err", err,
		)
	}
	realtimeUserMessage := protocol.Clone(e.userMessage)
	if clientMessageID := strings.TrimSpace(e.request.ClientMessageID); clientMessageID != "" {
		realtimeUserMessage["client_message_id"] = clientMessageID
	}
	e.service.broadcastSharedEvent(
		e.ctx,
		e.sessionKey,
		e.roomID,
		roomdomain.WrapMessageEvent(e.roomID, e.conversationID, realtimeUserMessage, e.request.RoundID),
	)
	return true
}
