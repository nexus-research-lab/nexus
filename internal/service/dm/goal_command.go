// INPUT: 已授权 `/goal` 或 UI set_goal、DM session 与 Goal service。
// OUTPUT: 被 runtime 截获前完成的 Goal 写入、durable 控制记录与 conversation draft 消费。
// POS: DM Goal command 的串行业务边界；不创建普通模型 round。
package dm

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// SetGoalFromCommand 设置 Goal 并持久化一条不进入 runtime 的用户控制记录。
func (s *Service) SetGoalFromCommand(
	ctx context.Context,
	request protocol.GoalCommandRequest,
) (protocol.GoalCommandResult, error) {
	if err := s.inputQueueDispatchMu.LockContext(ctx); err != nil {
		return protocol.GoalCommandResult{}, err
	}
	defer s.inputQueueDispatchMu.Unlock()

	execution, err := s.prepareChatExecution(ctx, Request{
		SessionKey:      request.SessionKey,
		AgentID:         request.AgentID,
		Content:         request.CommandContent,
		ClientRequestID: request.ClientRequestID,
		ClientMessageID: request.ClientMessageID,
		RoundID:         request.RoundID,
		UserMessageID:   request.UserMessageID,
		DeliveryPolicy:  protocol.ChatDeliveryPolicyAuto,
	})
	if err != nil {
		return protocol.GoalCommandResult{}, err
	}
	item, err := runtimehost.CreateGoalFromCommand(ctx, s.goals, request, execution.sessionKey, execution.request.RoundID, "")
	if err != nil {
		return protocol.GoalCommandResult{}, err
	}
	committed := s.persistGoalCommandRecord(ctx, execution, item)
	return protocol.GoalCommandResult{
		Goal:                 item,
		UserMessageCommitted: committed,
	}, nil
}

func (s *Service) persistGoalCommandRecord(
	ctx context.Context,
	execution *dmChatExecution,
	item protocol.Goal,
) bool {
	if execution == nil || execution.agent == nil {
		return false
	}
	now := time.Now().UTC()
	err := s.recordRoundMarkerWithOptionsForOwner(
		execution.agent.OwnerUserID,
		execution.agent.WorkspacePath,
		execution.session,
		execution.request.RoundID,
		strings.TrimSpace(execution.request.Content),
		workspacestore.RoundMarkerOptions{
			UserMessageID:   execution.request.UserMessageID,
			ClientMessageID: execution.request.ClientMessageID,
			DeliveryPolicy:  string(protocol.ChatDeliveryPolicyAuto),
			Purpose:         "goal_command",
			ControlOnly:     true,
			Metadata: map[string]string{
				"subtype":                 "goal_set",
				"goal_id":                 item.ID,
				"goal_objective_revision": strconv.FormatInt(item.ObjectiveRevision(), 10),
			},
		},
	)
	if err != nil {
		s.LoggerFor(ctx).Error("Goal 已设置，但 DM 控制记录持久化失败",
			"session_key", execution.sessionKey,
			"goal_id", item.ID,
			"round_id", execution.request.RoundID,
			"err", err,
		)
		return false
	}
	if dmRoomConversationID(execution.parsed) != "" {
		if err = s.markRoomConversationStarted(ctx, execution.sessionKey, now); err != nil {
			s.LoggerFor(ctx).Warn("Goal 控制记录已持久化，但 conversation draft 状态更新失败",
				"session_key", execution.sessionKey,
				"goal_id", item.ID,
				"err", err,
			)
		}
	}
	if _, err = s.refreshSessionMetaAfterRoundMarkerForOwner(
		execution.agent.OwnerUserID,
		execution.agent.WorkspacePath,
		execution.session,
	); err != nil {
		s.LoggerFor(ctx).Warn("Goal 控制记录已持久化，但 session meta 更新失败",
			"session_key", execution.sessionKey,
			"goal_id", item.ID,
			"err", err,
		)
	}
	return true
}
