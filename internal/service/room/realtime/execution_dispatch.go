// INPUT: orchestration Assignment target preflight、current Spec/accepted dependency WorkContract、structured Dispatch 与 Room context。
// OUTPUT: 不依赖 @ 文本、带可执行输入的 slot/queue 投递，以及 managed Execution target admission。
// POS: Execution outbox 到 Room realtime 数据面的消费侧适配器。
package realtime

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type executionTargetAuthorizer interface {
	AuthorizeRoomRuntimeTarget(
		context.Context,
		orchestrationsvc.ActorContext,
		*protocol.ExecutionWorkBinding,
	) error
}

type executionAttemptActivator interface {
	ActivateRoomAttempt(
		context.Context,
		orchestrationsvc.ActorContext,
		orchestrationsvc.RoomAttemptActivationInput,
	) error
}

type executionAttemptTerminalizer interface {
	FinishRoomAttempt(
		context.Context,
		orchestrationsvc.ActorContext,
		orchestrationsvc.RoomAttemptTerminalInput,
	) error
}

// AuthorizeAssignmentTarget 在 Assignment 写库前验证目标是当前 Room 的 Agent member。
func (s *Service) AuthorizeAssignmentTarget(
	ctx context.Context,
	request orchestrationsvc.AssignmentTargetRequest,
) error {
	if s == nil || s.rooms == nil {
		return errors.New("Room context store is unavailable")
	}
	contextValue, err := s.rooms.GetConversationContextForSystem(ctx, strings.TrimSpace(request.ConversationID))
	if err != nil {
		return err
	}
	return authorizeAssignmentTargetContext(contextValue, request)
}

func authorizeAssignmentTargetContext(
	contextValue *protocol.ConversationContextAggregate,
	request orchestrationsvc.AssignmentTargetRequest,
) error {
	if err := requireGroupRoomContext(contextValue); err != nil {
		return err
	}
	if strings.TrimSpace(request.OwnerUserID) != strings.TrimSpace(contextValue.Room.OwnerUserID) ||
		strings.TrimSpace(request.RoomID) != strings.TrimSpace(contextValue.Room.ID) ||
		strings.TrimSpace(request.ConversationID) != strings.TrimSpace(contextValue.Conversation.ID) ||
		strings.TrimSpace(request.SessionKey) != protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID) {
		return errors.New("Assignment target request is outside the Room execution scope")
	}
	targetAgentID := strings.TrimSpace(request.TargetAgentID)
	if targetAgentID == "" || !roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
		return roomsvc.ErrRoomMemberNotFound
	}
	if _, ok := findRoomSessionForAgent(contextValue.Sessions, targetAgentID); !ok {
		return errors.New("target Room member has no runtime session")
	}
	return nil
}

// DeliverExecutionDispatch 同步接受一个已 claim 的 outbox item。
//
// Dispatch 先进入目标的 durable input queue，再由统一 Room delivery path
// 创建 slot。Binding 是权威关联，instruction 中是否包含 @ 不影响路由。
func (s *Service) DeliverExecutionDispatch(
	ctx context.Context,
	delivery orchestrationsvc.ExecutionDispatchDelivery,
) (orchestrationsvc.ExecutionDispatchReceipt, error) {
	var receipt orchestrationsvc.ExecutionDispatchReceipt
	if err := validateExecutionDispatchDelivery(delivery); err != nil {
		return receipt, orchestrationsvc.PermanentExecutionDispatchDelivery(err)
	}
	if delivery.Kind != protocol.ExecutionDispatchRoomDirected &&
		delivery.Kind != protocol.ExecutionDispatchRoomPublic {
		return receipt, orchestrationsvc.PermanentExecutionDispatchDelivery(
			errors.New("Room delivery requires a Room Dispatch kind"),
		)
	}
	contextValue, err := s.rooms.GetConversationContextForSystem(ctx, delivery.ConversationID)
	if err != nil {
		return receipt, err
	}
	if err = requireGroupRoomContext(contextValue); err != nil {
		return receipt, orchestrationsvc.PermanentExecutionDispatchDelivery(err)
	}
	if err = authorizeAssignmentTargetContext(
		contextValue,
		orchestrationsvc.AssignmentTargetRequest{
			OwnerUserID:    delivery.OwnerUserID,
			SessionKey:     delivery.SessionKey,
			ExecutionID:    delivery.Binding.ExecutionID,
			RoomID:         delivery.RoomID,
			ConversationID: delivery.ConversationID,
			ActorAgentID:   delivery.SourceAgentID,
			TargetAgentID:  delivery.TargetAgentID,
			Strategy:       protocol.AssignmentStrategyRoomMember,
		},
	); err != nil {
		return receipt, orchestrationsvc.PermanentExecutionDispatchDelivery(err)
	}
	handoffID := "execution_dispatch_" + delivery.Binding.DispatchID
	content := renderExecutionDispatchInstruction(delivery)
	accepted, existingReceipt, err := s.ensureExecutionDispatchHandoff(
		delivery,
		handoffID,
		content,
	)
	if err != nil {
		return receipt, err
	}
	if accepted {
		return existingReceipt, nil
	}
	parentRound := &activeRoomRound{
		SessionKey:         delivery.SessionKey,
		RoomID:             contextValue.Room.ID,
		ConversationID:     contextValue.Conversation.ID,
		CoordinatorAgentID: strings.TrimSpace(delivery.SourceAgentID),
		RoomType:           contextValue.Room.RoomType,
		Context:            contextValue,
		RoundID:            "execution_dispatch_" + delivery.Binding.DispatchID,
		RootRoundID:        "execution_dispatch_" + delivery.Binding.DispatchID,
		OwnerUserID:        contextValue.Room.OwnerUserID,
	}
	if err = s.authorizeManagedExecutionTarget(
		ctx,
		parentRound,
		delivery.TargetAgentID,
		&delivery.Binding,
	); err != nil {
		return receipt, err
	}
	return s.enqueueExecutionDispatch(
		ctx,
		contextValue,
		parentRound,
		delivery,
		handoffID,
		content,
	)
}

func (s *Service) enqueueExecutionDispatch(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	parentRound *activeRoomRound,
	delivery orchestrationsvc.ExecutionDispatchDelivery,
	handoffID string,
	content string,
) (orchestrationsvc.ExecutionDispatchReceipt, error) {
	var receipt orchestrationsvc.ExecutionDispatchReceipt
	if s == nil || s.inputQueue == nil || contextValue == nil || parentRound == nil {
		return receipt, errors.New("durable Room input queue is unavailable")
	}
	delivery.TargetAgentID = strings.TrimSpace(delivery.TargetAgentID)
	delivery.SourceAgentID = strings.TrimSpace(delivery.SourceAgentID)
	delivery.OwnerUserID = strings.TrimSpace(delivery.OwnerUserID)
	locations, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return receipt, err
	}
	location, ok := locations[delivery.TargetAgentID]
	if !ok {
		return receipt, orchestrationsvc.PermanentExecutionDispatchDelivery(
			errors.New("Execution Dispatch target has no durable queue location"),
		)
	}
	item := protocol.InputQueueItem{
		ID:              handoffID,
		Scope:           protocol.InputQueueScopeRoom,
		SessionKey:      location.Location.SessionKey,
		RoomID:          delivery.RoomID,
		ConversationID:  delivery.ConversationID,
		AgentID:         delivery.TargetAgentID,
		SourceAgentID:   delivery.SourceAgentID,
		SourceMessageID: handoffID,
		HandoffID:       handoffID,
		TargetAgentIDs:  []string{delivery.TargetAgentID},
		Source:          protocol.InputQueueSourceAgentRoomMessage,
		Content:         content,
		DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
		OwnerUserID:     delivery.OwnerUserID,
		RootRoundID:     roomRootRoundID(parentRound),
		WorkBinding:     cloneExecutionWorkBinding(&delivery.Binding),
	}
	items, inserted, err := s.inputQueue.EnqueueBounded(location.Location, item, 0)
	if err != nil {
		return receipt, err
	}
	if !inserted {
		for _, existing := range items {
			if existing.ID == handoffID ||
				strings.TrimSpace(existing.HandoffID) == handoffID {
				item = existing
				break
			}
		}
		if !executionWorkBindingEqual(item.WorkBinding, &delivery.Binding) {
			return receipt, orchestrationsvc.PermanentExecutionDispatchDelivery(
				errors.New("Execution Dispatch queue identity conflicts with durable Room state"),
			)
		}
	}
	if err = s.publicHandoffs.MarkQueued(
		delivery.OwnerUserID,
		delivery.ConversationID,
		handoffID,
		item.ID,
	); err != nil {
		return receipt, err
	}
	if err = s.broadcastRoomInputQueueSnapshot(
		ctx,
		delivery.SessionKey,
		contextValue,
	); err != nil {
		s.loggerFor(ctx).Warn(
			"广播 Execution Dispatch 队列快照失败",
			"dispatch_id",
			delivery.Binding.DispatchID,
			"err",
			err,
		)
	}
	if len(s.findActiveDeliverySlotsByAgent(
		delivery.SessionKey,
		delivery.ConversationID,
		[]string{delivery.TargetAgentID},
	)) == 0 {
		s.startSessionBackgroundTask(
			delivery.SessionKey,
			delivery.OwnerUserID,
			func(taskCtx context.Context) {
				s.dispatchNextInputQueueItem(
					taskCtx,
					delivery.SessionKey,
					delivery.RoomID,
					delivery.ConversationID,
				)
			},
		)
	}
	return orchestrationsvc.ExecutionDispatchReceipt{
		HandoffID:   handoffID,
		QueueItemID: item.ID,
	}, nil
}

func (s *Service) ensureExecutionDispatchHandoff(
	delivery orchestrationsvc.ExecutionDispatchDelivery,
	handoffID string,
	content string,
) (bool, orchestrationsvc.ExecutionDispatchReceipt, error) {
	var receipt orchestrationsvc.ExecutionDispatchReceipt
	if s == nil || s.publicHandoffs == nil {
		return false, receipt, errors.New("durable Room handoff store is unavailable")
	}
	binding := delivery.Binding
	handoff, inserted, err := s.publicHandoffs.Detect(
		delivery.OwnerUserID,
		workspacestore.RoomPublicHandoff{
			HandoffID:          handoffID,
			ConversationID:     delivery.ConversationID,
			RoomID:             delivery.RoomID,
			RootRoundID:        handoffID,
			SourceAgentRoundID: handoffID,
			SourceMessageID:    handoffID,
			SourceAgentID:      delivery.SourceAgentID,
			TargetAgentID:      delivery.TargetAgentID,
			Content:            content,
			WorkBinding:        &binding,
		},
	)
	if err != nil {
		return false, receipt, err
	}
	if !executionWorkBindingEqual(handoff.WorkBinding, &binding) ||
		strings.TrimSpace(handoff.SourceAgentID) != strings.TrimSpace(delivery.SourceAgentID) ||
		strings.TrimSpace(handoff.TargetAgentID) != strings.TrimSpace(delivery.TargetAgentID) {
		return false, receipt, orchestrationsvc.PermanentExecutionDispatchDelivery(
			errors.New("Execution Dispatch handoff identity conflicts with durable Room state"),
		)
	}
	if inserted || strings.TrimSpace(handoff.Status) == "detected" {
		if err = s.publicHandoffs.MarkSourceFinished(
			delivery.OwnerUserID,
			delivery.ConversationID,
			handoffID,
		); err != nil {
			return false, receipt, err
		}
		handoff.Status = "source_finished"
	}
	if executionDispatchHandoffAccepted(handoff.Status) {
		return true, orchestrationsvc.ExecutionDispatchReceipt{
			HandoffID:   handoffID,
			QueueItemID: strings.TrimSpace(handoff.QueueItemID),
		}, nil
	}
	return false, receipt, nil
}

func executionDispatchHandoffAccepted(status string) bool {
	switch strings.TrimSpace(status) {
	case "queued", "started", "finished", "error", "interrupted":
		return true
	default:
		return false
	}
}

func executionWorkBindingEqual(
	left *protocol.ExecutionWorkBinding,
	right *protocol.ExecutionWorkBinding,
) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return strings.TrimSpace(left.ExecutionID) == strings.TrimSpace(right.ExecutionID) &&
		strings.TrimSpace(left.PlanID) == strings.TrimSpace(right.PlanID) &&
		strings.TrimSpace(left.WorkItemID) == strings.TrimSpace(right.WorkItemID) &&
		strings.TrimSpace(left.SpecID) == strings.TrimSpace(right.SpecID) &&
		strings.TrimSpace(left.AssignmentID) == strings.TrimSpace(right.AssignmentID) &&
		strings.TrimSpace(left.AttemptID) == strings.TrimSpace(right.AttemptID) &&
		strings.TrimSpace(left.DispatchID) == strings.TrimSpace(right.DispatchID)
}

func (s *Service) authorizeManagedExecutionTarget(
	ctx context.Context,
	roundValue *activeRoomRound,
	targetAgentID string,
	binding *protocol.ExecutionWorkBinding,
) error {
	// 没有 binding 的用户消息与裸 @ 永远属于 conversation lane；Room 中
	// 同时存在 active Execution 也不能把通信隐式升级成责任。
	if binding == nil {
		return nil
	}
	if s == nil || s.executionContext == nil || roundValue == nil {
		return errors.New("managed Execution target admission is unavailable")
	}
	authorizer, ok := s.executionContext.(executionTargetAuthorizer)
	if !ok {
		// 配置了 managed Execution context 却没有 admission 能力时 fail closed；
		// 不能让 raw @ 绕过 WorkGraph。
		return errors.New("managed Execution target admission is unavailable")
	}
	return authorizer.AuthorizeRoomRuntimeTarget(ctx, orchestrationsvc.ActorContext{
		OwnerUserID:    roundValue.OwnerUserID,
		SessionKey:     roundValue.SessionKey,
		ExecutionID:    executionIDFromWorkBinding(binding),
		AgentID:        strings.TrimSpace(targetAgentID),
		ActorKind:      protocol.ExecutionActorAgent,
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
		RootRoundID:    roomRootRoundID(roundValue),
	}, binding)
}

func (e *slotExecution) activateBoundRoomAttempt(actor orchestrationsvc.ActorContext) error {
	if e == nil || e.slot == nil || e.slot.WorkBinding == nil {
		return nil
	}
	if e.service == nil || e.service.executionContext == nil {
		return errors.New("managed Execution Attempt activation is unavailable")
	}
	activator, ok := e.service.executionContext.(executionAttemptActivator)
	if !ok {
		return errors.New("managed Execution Attempt activator is unavailable")
	}
	return activator.ActivateRoomAttempt(e.ctx, actor, orchestrationsvc.RoomAttemptActivationInput{
		Binding:           *cloneExecutionWorkBinding(e.slot.WorkBinding),
		RuntimeSessionKey: e.slot.RuntimeSessionKey,
		RoomSessionID:     e.slot.RoomSessionID,
	})
}

func validateExecutionDispatchDelivery(delivery orchestrationsvc.ExecutionDispatchDelivery) error {
	values := map[string]string{
		"owner_user_id":   delivery.OwnerUserID,
		"session_key":     delivery.SessionKey,
		"room_id":         delivery.RoomID,
		"conversation_id": delivery.ConversationID,
		"source_agent_id": delivery.SourceAgentID,
		"target_agent_id": delivery.TargetAgentID,
		"instruction":     delivery.Instruction,
		"execution_id":    delivery.Binding.ExecutionID,
		"plan_id":         delivery.Binding.PlanID,
		"work_item_id":    delivery.Binding.WorkItemID,
		"spec_id":         delivery.Binding.SpecID,
		"assignment_id":   delivery.Binding.AssignmentID,
		"attempt_id":      delivery.Binding.AttemptID,
		"dispatch_id":     delivery.Binding.DispatchID,
	}
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

func renderExecutionDispatchInstruction(delivery orchestrationsvc.ExecutionDispatchDelivery) string {
	binding := delivery.Binding
	var output strings.Builder
	fmt.Fprintf(
		&output,
		"[Nexus structured work assignment]\n"+
			"execution_id: %s\nplan_id: %s\nwork_item_id: %s\nspec_id: %s\n"+
			"assignment_id: %s\nattempt_id: %s\ndispatch_id: %s\n",
		binding.ExecutionID,
		binding.PlanID,
		binding.WorkItemID,
		binding.SpecID,
		binding.AssignmentID,
		binding.AttemptID,
		binding.DispatchID,
	)
	output.WriteString("\n[work_contract]\ninput_refs:")
	for _, ref := range delivery.WorkContract.InputRefs {
		fmt.Fprintf(&output, "\n- %s", strconv.Quote(strings.TrimSpace(ref)))
	}
	output.WriteString("\noutput_scopes:")
	for _, scope := range delivery.WorkContract.OutputScopes {
		fmt.Fprintf(
			&output,
			"\n- mode=%s scope=%s",
			scope.Mode,
			strconv.Quote(strings.TrimSpace(scope.Scope)),
		)
	}
	output.WriteString("\naccepted_dependencies:")
	for _, dependency := range delivery.WorkContract.AcceptedDependencies {
		fmt.Fprintf(
			&output,
			"\n- work_item_id=%s logical_key=%s spec_id=%s kind=%s submission_id=%s acceptance_id=%s",
			dependency.WorkItemID,
			strconv.Quote(strings.TrimSpace(dependency.LogicalKey)),
			dependency.SpecID,
			dependency.Kind,
			dependency.SubmissionID,
			dependency.AcceptanceID,
		)
		fmt.Fprintf(
			&output,
			"\n  result_summary: %s",
			strconv.Quote(strings.TrimSpace(dependency.ResultSummary)),
		)
		output.WriteString("\n  result_refs:")
		for _, ref := range dependency.ResultRefs {
			fmt.Fprintf(&output, "\n  - %s", strconv.Quote(strings.TrimSpace(ref)))
		}
	}
	fmt.Fprintf(
		&output,
		"\n\n[instruction]\n%s",
		strings.TrimSpace(delivery.Instruction),
	)
	return output.String()
}

func executionIDFromWorkBinding(binding *protocol.ExecutionWorkBinding) string {
	if binding == nil {
		return ""
	}
	return strings.TrimSpace(binding.ExecutionID)
}

func executionIDFromRoomBindings(
	workBinding *protocol.ExecutionWorkBinding,
	reviewBinding *protocol.ExecutionReviewBinding,
) string {
	if workBinding != nil {
		return strings.TrimSpace(workBinding.ExecutionID)
	}
	if reviewBinding != nil {
		return strings.TrimSpace(reviewBinding.ExecutionID)
	}
	return ""
}

func executionDispatchID(binding *protocol.ExecutionWorkBinding) string {
	if binding == nil {
		return ""
	}
	return strings.TrimSpace(binding.DispatchID)
}

func cloneExecutionWorkBinding(binding *protocol.ExecutionWorkBinding) *protocol.ExecutionWorkBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}

func cloneExecutionReviewBinding(
	binding *protocol.ExecutionReviewBinding,
) *protocol.ExecutionReviewBinding {
	if binding == nil {
		return nil
	}
	result := *binding
	return &result
}

func roomCoordinatorAgentID(explicit string, contextValue *protocol.ConversationContextAggregate) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	if contextValue == nil {
		return ""
	}
	return strings.TrimSpace(contextValue.Room.HostAgentID)
}

func firstRoomTargetAgentID(values []string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func roomExecutionActorRole(coordinatorAgentID string, actorAgentID string) orchestrationsvc.ExecutionActorRole {
	if coordinatorAgentID = strings.TrimSpace(coordinatorAgentID); coordinatorAgentID != "" &&
		coordinatorAgentID == strings.TrimSpace(actorAgentID) {
		return orchestrationsvc.ExecutionActorCoordinator
	}
	// Host/lead 缺失时 fail closed；不能让任意第一个 slot 成为 coordinator。
	return orchestrationsvc.ExecutionActorMember
}
