// INPUT: 持久化 ScheduledTask source/delivery、当前 Agent 身份、统一 Session 与 Room 成员事实。
// OUTPUT: 真实接收 Session 校验、Agent-origin 投递授权与明确拒绝。
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
	if isLegacyAutomationInboxDelivery(task.Delivery) {
		return errors.New("the scheduled task inbox is legacy-only; select an existing Nexus, Room, or IM session")
	}
	if err := validateConfigurableDeliveryTarget(task.Delivery); err != nil {
		return err
	}
	if err := s.resolveRoomDeliveryAgent(ctx, task); err != nil {
		return err
	}
	normalizedGrantSource := automationdomain.Source{}
	if grantSource != nil {
		normalizedGrantSource = grantSource.Normalized()
	}
	actorAgentID, agentActor := automationexec.ActorAgentID(ctx)
	if agentActor {
		if normalizedGrantSource.Kind != automationdomain.SourceKindAgent {
			return errors.New("Agent-origin automation mutation must use the trusted Agent source")
		}
		if normalizedGrantSource.CreatorAgentID != strings.TrimSpace(actorAgentID) {
			return errors.New("automation source creator does not match the trusted Agent actor")
		}
		task.DeliveryGrant = normalizedGrantSource
		if err := s.validateAgentOriginDeliveryGrant(ctx, *task); err != nil {
			return err
		}
		return s.validatePersistentDeliveryGrant(ctx, *task)
	}
	kind := automationdomain.SourceKindUserPage
	if normalizedGrantSource.Kind != "" {
		switch normalizedGrantSource.Kind {
		case automationdomain.SourceKindUserPage, automationdomain.SourceKindCLI:
			kind = normalizedGrantSource.Kind
		}
	}
	task.DeliveryGrant = automationdomain.Source{Kind: kind}.Normalized()
	return s.validatePersistentDeliveryGrant(ctx, *task)
}

func isLegacyAutomationInboxDelivery(target automationdomain.DeliveryTarget) bool {
	normalized := target.Normalized()
	sessionKey := normalized.SessionKey
	if sessionKey == "" && protocol.ParseSessionKey(normalized.To).IsStructured {
		sessionKey = normalized.To
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	return parsed.IsStructured &&
		parsed.Kind == protocol.SessionKeyKindAgent &&
		protocol.NormalizeStoredChannelType(parsed.Channel) == protocol.SessionChannelInternalSegment &&
		strings.TrimSpace(parsed.Ref) == protocol.AutomationInboxSessionRef
}

// validateConfigurableDeliveryTarget 要求所有新建或改绑都指向结构化 Session。
// 裸 channel/to 与不带 SessionKey 的 last 只属于已持久化旧任务的运行兼容。
func validateConfigurableDeliveryTarget(target automationdomain.DeliveryTarget) error {
	normalized := target.Normalized()
	if normalized.Mode == automationdomain.DeliveryModeNone {
		return nil
	}
	// `last` without a key is the long-standing "use the remembered route" runtime
	// contract. It does not create a Session and remains valid for non-UI callers.
	if normalized.Mode == automationdomain.DeliveryModeLast &&
		normalized.SessionKey == "" &&
		normalized.To == "" {
		return nil
	}
	sessionKey := normalized.SessionKey
	if sessionKey == "" && protocol.ParseSessionKey(normalized.To).IsStructured {
		sessionKey = normalized.To
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured ||
		(parsed.Kind != protocol.SessionKeyKindAgent && parsed.Kind != protocol.SessionKeyKindRoom) {
		return automationdomain.ErrTaskDeliverySessionUnavailable
	}
	return nil
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
	if strings.TrimSpace(job.DeletionState) != "" {
		return automationdomain.ScheduledTask{}, automationdomain.ErrTaskDeleting
	}
	if job.SessionBindingState == automationdomain.TaskSessionBindingStateRebindRequired {
		return automationdomain.ScheduledTask{}, automationdomain.ErrTaskSessionRebindRequired
	}
	if job.Delivery.Normalized().Mode == automationdomain.DeliveryModeNone {
		return job, nil
	}
	job.Delivery = target.Normalized()
	if err := s.resolveRoomDeliveryAgent(ctx, &job); err != nil {
		return automationdomain.ScheduledTask{}, err
	}
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

// resolveRoomDeliveryAgent 把 Room 的“默认房主”选择解析成持久化的精确 Agent，
// 并在配置写入、运行和重试阶段复核同 owner、同 conversation 的当前成员身份。
// 非 Room Session 不接受独立的 AgentID，避免把回复身份偷渡到 DM/IM 路由。
func (s *Service) resolveRoomDeliveryAgent(
	ctx context.Context,
	job *automationdomain.ScheduledTask,
) error {
	if job == nil {
		return errors.New("automation delivery validation requires a task")
	}
	target := job.Delivery.Normalized()
	if target.Mode == automationdomain.DeliveryModeNone {
		target.AgentID = ""
		job.Delivery = target
		return nil
	}
	sessionKey := strings.TrimSpace(target.SessionKey)
	if sessionKey == "" && protocol.ParseSessionKey(target.To).IsStructured {
		sessionKey = strings.TrimSpace(target.To)
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindRoom {
		if target.AgentID != "" {
			return errors.New("delivery.agent_id is only valid for a Room session")
		}
		job.Delivery = target
		return nil
	}
	if s.room == nil {
		return errors.New("Room automation delivery cannot revalidate current membership")
	}
	contextValue, err := s.room.GetConversationContext(ctx, parsed.ConversationID)
	if err != nil {
		return err
	}
	if contextValue == nil || contextValue.Room.RoomType != protocol.RoomTypeGroup {
		return errors.New("Automation delivery Room is no longer available")
	}
	if target.AgentID == "" {
		target.AgentID = strings.TrimSpace(contextValue.Room.HostAgentID)
	}
	if target.AgentID == "" {
		return errors.New("Room automation delivery requires a replying Agent or Room host")
	}
	if !isAvailableRoomSessionAgent(contextValue, target.AgentID) {
		return errors.New("Automation delivery replying Agent is no longer a member of the target Room session")
	}
	if _, err = s.requireSameOwnerDeliveryAgent(ctx, *job, target.AgentID); err != nil {
		return err
	}
	job.Delivery = target
	return nil
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
	if parsed.IsStructured && parsed.Kind == protocol.SessionKeyKindRoom {
		return s.validateRoomDeliveryMembership(ctx, job)
	}
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return automationdomain.ErrTaskDeliverySessionUnavailable
	}
	targetAgentID := strings.TrimSpace(parsed.AgentID)
	if targetAgentID == "" {
		return errors.New("automation delivery session is missing its target Agent")
	}
	targetAgent, err := s.requireSameOwnerDeliveryAgent(ctx, job, targetAgentID)
	if err != nil {
		return err
	}
	if err = s.validateExistingAgentDeliverySession(ctx, targetAgent, sessionKey); err != nil {
		return err
	}
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	if channel == protocol.SessionChannelWebSocket || channel == protocol.SessionChannelInternalSegment {
		return nil
	}
	if s.deliveryGrants == nil {
		return errors.New("automation IM delivery grant resolver is not configured")
	}
	err = s.deliveryGrants.ValidateAutomationDeliveryGrant(
		ctx,
		strings.TrimSpace(job.OwnerUserID),
		targetAgentID,
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
		"agent_id", targetAgentID,
		"job_id", strings.TrimSpace(job.JobID),
		"err", err,
	)
	return automationdomain.ErrTaskDeliverySessionUnavailable
}

// validateExistingAgentDeliverySession 保证新建/改绑只引用真实存在的 Nexus 会话。
// 历史 automation-inbox 的运行兼容由 channels 投递层承担，配置写入路径不会进入这里。
func (s *Service) validateExistingAgentDeliverySession(
	ctx context.Context,
	targetAgent *protocol.Agent,
	sessionKey string,
) error {
	// 部分纯领域测试不装配 Agent store；生产服务始终有 targetAgent，仍在实际投递层
	// 再次 fail closed。这里不为测试伪造 workspace 或 Session。
	if targetAgent == nil {
		return nil
	}
	if s.deliverySessions == nil {
		return automationdomain.ErrTaskDeliverySessionUnavailable
	}
	stored, err := s.deliverySessions.ResolveDeliverySession(ctx, strings.TrimSpace(sessionKey))
	if err != nil {
		return automationdomain.ErrTaskDeliverySessionUnavailable
	}
	if stored == nil ||
		strings.TrimSpace(stored.SessionKey) != strings.TrimSpace(sessionKey) ||
		strings.TrimSpace(stored.AgentID) != strings.TrimSpace(targetAgent.AgentID) {
		return automationdomain.ErrTaskDeliverySessionUnavailable
	}
	return nil
}

func (s *Service) requireSameOwnerDeliveryAgent(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	agentID string,
) (*protocol.Agent, error) {
	if s.agents == nil {
		return nil, nil
	}
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if agentValue == nil ||
		(strings.TrimSpace(job.OwnerUserID) != "" &&
			strings.TrimSpace(agentValue.OwnerUserID) != strings.TrimSpace(job.OwnerUserID)) {
		return nil, errors.New("automation delivery target Agent must be owned by the task owner")
	}
	return agentValue, nil
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
	targetSessionKey := strings.TrimSpace(target.SessionKey)
	if targetSessionKey == "" {
		targetSessionKey = strings.TrimSpace(target.To)
	}
	targetSession := protocol.ParseSessionKey(targetSessionKey)
	if targetSession.Kind != protocol.SessionKeyKindRoom {
		return nil
	}
	grant := job.DeliveryGrant.Normalized()
	if s.room == nil {
		return errors.New("Room automation delivery cannot revalidate current membership")
	}
	contextValue, err := s.room.GetConversationContext(ctx, targetSession.ConversationID)
	if err != nil {
		return err
	}
	if contextValue == nil {
		return errors.New("Automation delivery Room is no longer available")
	}
	if !isAvailableRoomSessionAgent(contextValue, target.AgentID) {
		return errors.New("Automation delivery replying Agent is no longer a member of the target Room session")
	}
	// 人类控制面可以把同 owner Agent 的产物投递到另一个 Room；Room 是结果接收方，
	// 不是执行身份。普通 Agent 自主创建的任务仍只能回到可信来源 Room，并且执行
	// Agent 必须保持成员身份，防止模型借定时任务跨 Room 注入消息。
	if grant.Kind != automationdomain.SourceKindAgent {
		return nil
	}
	sourceSession := protocol.ParseSessionKey(grant.SessionKey)
	if sourceSession.Kind != protocol.SessionKeyKindRoom ||
		sourceSession.ConversationID == "" ||
		sourceSession.Raw != targetSession.Raw {
		return errors.New("Room automation delivery is not bound to its trusted source conversation")
	}
	if !roomdomain.IsMemberAgent(contextValue.Members, strings.TrimSpace(job.AgentID)) {
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
