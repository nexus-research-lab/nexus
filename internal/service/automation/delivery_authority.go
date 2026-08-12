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
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

// prepareTaskDeliveryMutation 把 delivery grant 固定到本次真实调用方，同时保持
// Source 作为不可变的创建 provenance。
//
// Agent 调用必须携带 MCP 从可信当前会话生成的 Source；人类 HTTP/CLI 修改投递
// 属于显式控制面 grant，不需要伪造最初创建任务的 Agent actor。
func (s *Service) prepareTaskDeliveryMutation(
	ctx context.Context,
	task *automationdomain.ScheduledTask,
	grantSource *automationdomain.Source,
) error {
	if task == nil {
		return errors.New("automation delivery validation requires a task")
	}
	actorAgentID, agentActor := automationexec.ActorAgentID(ctx)
	if agentActor {
		if grantSource == nil || strings.TrimSpace(grantSource.Kind) != automationdomain.SourceKindAgent {
			return errors.New("Agent-origin automation mutation must use the trusted Agent source")
		}
		if strings.TrimSpace(grantSource.CreatorAgentID) != strings.TrimSpace(actorAgentID) {
			return errors.New("automation source creator does not match the trusted Agent actor")
		}
		task.DeliveryGrant = grantSource.Normalized()
		if err := s.validateAgentOriginDeliveryGrant(ctx, *task); err != nil {
			return err
		}
		return s.validatePersistentDeliveryGrant(ctx, *task)
	}
	kind := automationdomain.SourceKindUserPage
	if grantSource != nil {
		switch strings.TrimSpace(grantSource.Kind) {
		case automationdomain.SourceKindUserPage, automationdomain.SourceKindCLI:
			kind = strings.TrimSpace(grantSource.Kind)
		}
	}
	task.DeliveryGrant = automationdomain.Source{Kind: kind}.Normalized()
	return s.validatePersistentDeliveryGrant(ctx, *task)
}

// authorizedDeliveryJob 重读最新任务，再验证 Agent-origin 的 owner/self/Room 权限。
// 运行开始后的配置更新或权限撤销因此不会使用旧 job 快照投递。
func (s *Service) authorizedDeliveryJob(
	ctx context.Context,
	snapshot automationdomain.ScheduledTask,
) (automationdomain.ScheduledTask, error) {
	return s.authorizedDeliveryJobForTarget(ctx, snapshot, snapshot.Delivery)
}

// authorizedDeliveryJobForTarget 重读当前任务状态，但按 run 开始时冻结的逻辑目标
// 复核权限。显式关闭任务投递会立即生效，普通路由编辑不会重定向已在运行的结果。
func (s *Service) authorizedDeliveryJobForTarget(
	ctx context.Context,
	snapshot automationdomain.ScheduledTask,
	target automationdomain.DeliveryTarget,
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
	job = automationdomain.NormalizeScheduledTaskCompatibility(job)
	if job.SessionBindingState == automationdomain.TaskSessionBindingStateRebindRequired {
		return automationdomain.ScheduledTask{}, automationdomain.ErrTaskSessionRebindRequired
	}
	if job.Delivery.Normalized().Mode == automationdomain.DeliveryModeNone {
		return job, nil
	}
	job.Delivery = target.Normalized()
	if strings.TrimSpace(job.DeliveryGrant.Kind) == automationdomain.SourceKindAgent {
		if err := s.validateAgentOriginDeliveryGrant(contextForJobOwner(ctx, job), job); err != nil {
			return automationdomain.ScheduledTask{}, err
		}
	}
	if err := s.validatePersistentDeliveryGrant(contextForJobOwner(ctx, job), job); err != nil {
		return automationdomain.ScheduledTask{}, err
	}
	return job, nil
}

func (s *Service) validatePersistentDeliveryGrant(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) error {
	target := job.Delivery.Normalized()
	if target.Mode == automationdomain.DeliveryModeNone {
		return nil
	}
	sessionKey := strings.TrimSpace(target.SessionKey)
	if sessionKey == "" && protocol.ParseSessionKey(target.To).IsStructured {
		sessionKey = strings.TrimSpace(target.To)
	}
	// 旧任务的 explicit 外部目标没有 session_key；保留其既有控制面语义。
	// 新建 IM 任务统一写 last+session_key，只有新模型才由 active pairing 实时约束。
	if sessionKey == "" {
		return nil
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		channel := protocol.NormalizeStoredChannelType(target.Channel)
		if channel == "" || channel == protocol.SessionChannelWebSocket || channel == protocol.SessionChannelInternalSegment {
			return nil
		}
		return errors.New("external automation delivery requires a structured session_key grant")
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	if channel == protocol.SessionChannelWebSocket || channel == protocol.SessionChannelInternalSegment {
		return nil
	}
	if strings.TrimSpace(parsed.AgentID) != strings.TrimSpace(job.AgentID) {
		return errors.New("automation delivery session is bound to another Agent")
	}
	if s.deliveryGrants == nil {
		return errors.New("automation IM delivery grant resolver is not configured")
	}
	err := s.deliveryGrants.ValidateAutomationDeliveryGrant(
		ctx,
		strings.TrimSpace(job.OwnerUserID),
		strings.TrimSpace(job.AgentID),
		sessionKey,
	)
	if err == nil {
		return nil
	}
	if !errors.Is(err, channels.ErrExternalSessionGrantUnavailable) {
		return err
	}
	s.loggerFor(ctx).Warn(
		"定时任务 IM 投递目标未通过当前配对校验",
		"agent_id", strings.TrimSpace(job.AgentID),
		"job_id", strings.TrimSpace(job.JobID),
		"err", err,
	)
	return automationdomain.ErrTaskDeliverySessionUnavailable
}

func (s *Service) validateAgentOriginDeliveryGrant(
	ctx context.Context,
	job automationdomain.ScheduledTask,
) error {
	grant := job.DeliveryGrant.Normalized()
	creatorAgentID := strings.TrimSpace(grant.CreatorAgentID)
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
		grant.SessionKey,
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
	grant := job.DeliveryGrant.Normalized()
	if strings.TrimSpace(grant.ContextType) != "agent" ||
		strings.TrimSpace(grant.ContextID) != creatorAgentID {
		return false, nil
	}
	session := protocol.ParseSessionKey(grant.SessionKey)
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
	grant := job.DeliveryGrant.Normalized()
	sourceSession := protocol.ParseSessionKey(grant.SessionKey)
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
	sourceContextID := strings.TrimSpace(grant.ContextID)
	if sourceContextID != "" &&
		sourceContextID != strings.TrimSpace(contextValue.Room.ID) &&
		sourceContextID != strings.TrimSpace(contextValue.Conversation.ID) {
		return errors.New("Automation source Room no longer matches the granted conversation")
	}
	return nil
}
