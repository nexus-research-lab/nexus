// INPUT: 持久化 ScheduledTask source/delivery、当前 Agent 身份与最新 Room 成员事实。
// OUTPUT: Agent-origin 投递授权、并发更新后的最新可投递任务或明确拒绝。
// POS: Automation create/update 与实际 delivery/retry 的最终权限边界。
package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// prepareTaskDeliveryMutation 把 delivery 的授权来源固定到本次真实调用方。
//
// Agent 调用必须携带 MCP 从可信当前会话生成的 Source；人类 HTTP/CLI 调用没有
// ActorAgentID，属于显式控制面 grant，并改写为非 Agent 来源，避免后台把旧 Agent
// source 误当成仍然有效的 owner 权限。
func (s *Service) prepareTaskDeliveryMutation(
	ctx context.Context,
	task *automationdomain.ScheduledTask,
	deliveryChanged bool,
) error {
	if task == nil {
		return errors.New("automation delivery validation requires a task")
	}
	actorAgentID, agentActor := automationexec.ActorAgentID(ctx)
	sourceKind := strings.TrimSpace(task.Source.Kind)
	if agentActor {
		if sourceKind != automationdomain.SourceKindAgent {
			return errors.New("Agent-origin automation mutation must use the trusted Agent source")
		}
		if strings.TrimSpace(task.Source.CreatorAgentID) != strings.TrimSpace(actorAgentID) {
			return errors.New("automation source creator does not match the trusted Agent actor")
		}
		return s.validateAgentOriginDelivery(ctx, *task)
	}
	if deliveryChanged && sourceKind == automationdomain.SourceKindAgent {
		task.Source.Kind = automationdomain.SourceKindCLI
		task.Source.CreatorAgentID = ""
		task.Source = task.Source.Normalized()
		sourceKind = task.Source.Kind
	}
	if sourceKind != automationdomain.SourceKindAgent {
		return nil
	}
	return errors.New("Agent-origin automation mutation is missing the trusted actor context")
}

// authorizedDeliveryJob 重读最新任务，再验证 Agent-origin 的 owner/self/Room 权限。
// 运行开始后的配置更新或权限撤销因此不会使用旧 job 快照投递。
func (s *Service) authorizedDeliveryJob(
	ctx context.Context,
	snapshot automationdomain.ScheduledTask,
) (automationdomain.ScheduledTask, error) {
	job := snapshot
	if strings.TrimSpace(snapshot.JobID) != "" && snapshot.ConfigurationVersion > 0 {
		current, err := s.repository.GetScheduledTask(
			contextForJobOwner(ctx, snapshot),
			strings.TrimSpace(snapshot.OwnerUserID),
			strings.TrimSpace(snapshot.JobID),
		)
		if err != nil {
			return automationdomain.ScheduledTask{}, err
		}
		if current == nil {
			return automationdomain.ScheduledTask{}, automationdomain.ErrJobNotFound
		}
		job = *current
	}
	if job.Delivery.Normalized().Mode == automationdomain.DeliveryModeNone {
		return job, nil
	}
	if strings.TrimSpace(job.Source.Kind) != automationdomain.SourceKindAgent {
		return job, nil
	}
	if err := s.validateAgentOriginDelivery(contextForJobOwner(ctx, job), job); err != nil {
		return automationdomain.ScheduledTask{}, err
	}
	return job, nil
}

func (s *Service) validateAgentOriginDelivery(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) error {
	creatorAgentID := strings.TrimSpace(job.Source.CreatorAgentID)
	if creatorAgentID == "" {
		return errors.New("Agent-origin automation delivery is missing creator_agent_id")
	}
	ownerMain, err := s.isCurrentOwnerMainDeliveryGrant(ctx, job, creatorAgentID)
	if err != nil {
		return err
	}
	if ownerMain {
		return nil
	}
	if creatorAgentID != strings.TrimSpace(job.AgentID) {
		return fmt.Errorf(
			"Agent %s cannot grant automation delivery for task Agent %s",
			creatorAgentID,
			strings.TrimSpace(job.AgentID),
		)
	}
	if err = automationdomain.ValidateSelfScopedDeliveryTarget(
		job.AgentID,
		job.Source.SessionKey,
		job.Delivery,
	); err != nil {
		return err
	}
	return s.validateRoomDeliveryMembership(ctx, job)
}

func (s *Service) isCurrentOwnerMainDeliveryGrant(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	creatorAgentID string,
) (bool, error) {
	if strings.TrimSpace(job.Source.ContextType) != "agent" ||
		strings.TrimSpace(job.Source.ContextID) != creatorAgentID {
		return false, nil
	}
	session := protocol.ParseSessionKey(job.Source.SessionKey)
	if !session.IsStructured ||
		session.Kind != protocol.SessionKeyKindAgent ||
		session.Channel != protocol.SessionChannelWebSocketSegment ||
		session.ChatType != protocol.RoomTypeDM ||
		strings.TrimSpace(session.AgentID) != creatorAgentID {
		return false, nil
	}
	if s.agents == nil {
		return false, nil
	}
	creator, err := s.agents.GetAgent(ctx, creatorAgentID)
	if err != nil {
		return false, err
	}
	if creator == nil ||
		!creator.IsMain ||
		strings.TrimSpace(creator.OwnerUserID) == "" ||
		(strings.TrimSpace(job.OwnerUserID) != "" &&
			strings.TrimSpace(creator.OwnerUserID) != strings.TrimSpace(job.OwnerUserID)) {
		return false, nil
	}
	return true, nil
}

func (s *Service) validateRoomDeliveryMembership(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) error {
	target := job.Delivery.Normalized()
	if target.Mode != automationdomain.DeliveryModeExplicit ||
		protocol.NormalizeStoredChannelType(target.Channel) != protocol.SessionChannelWebSocket {
		return nil
	}
	targetSession := protocol.ParseSessionKey(target.To)
	if targetSession.Kind != protocol.SessionKeyKindRoom {
		return nil
	}
	sourceSession := protocol.ParseSessionKey(job.Source.SessionKey)
	if sourceSession.Kind != protocol.SessionKeyKindRoom ||
		sourceSession.ConversationID == "" ||
		sourceSession.Raw != targetSession.Raw {
		return errors.New("Room automation delivery is not bound to its trusted source conversation")
	}
	if s.room == nil {
		return errors.New("Room automation delivery cannot revalidate current membership")
	}
	contextValue, err := s.room.GetConversationContext(ctx, sourceSession.ConversationID)
	if err != nil {
		return err
	}
	if contextValue == nil ||
		!roomdomain.IsMemberAgent(contextValue.Members, strings.TrimSpace(job.AgentID)) {
		return errors.New("Automation delivery Agent is no longer a member of the granted Room")
	}
	sourceContextID := strings.TrimSpace(job.Source.ContextID)
	if sourceContextID != "" &&
		sourceContextID != strings.TrimSpace(contextValue.Room.ID) &&
		sourceContextID != strings.TrimSpace(contextValue.Conversation.ID) {
		return errors.New("Automation source Room no longer matches the granted conversation")
	}
	return nil
}
