// INPUT: claimed ExecutionReviewDispatch、trusted review binding 与 Room context。
// OUTPUT: 不依赖 @ 文本的 reviewer durable handoff/queue 投递。
// POS: Submission review return 的 Room 数据面；不创建 worker Assignment 或 reviewer Attempt。
package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type executionReviewTargetAuthorizer interface {
	AuthorizeRoomReviewReturn(
		context.Context,
		orchestrationsvc.ActorContext,
		*protocol.ExecutionReviewBinding,
	) error
}

// DeliverExecutionReviewDispatch 同步接受一个已 claim 的 review-return outbox item。
func (s *Service) DeliverExecutionReviewDispatch(
	ctx context.Context,
	delivery orchestrationsvc.ExecutionReviewDispatchDelivery,
) (orchestrationsvc.ExecutionReviewDispatchReceipt, error) {
	var receipt orchestrationsvc.ExecutionReviewDispatchReceipt
	if err := validateExecutionReviewDispatchDelivery(delivery); err != nil {
		return receipt, err
	}
	contextValue, err := s.rooms.GetConversationContextForSystem(
		ctx,
		delivery.ConversationID,
	)
	if err != nil {
		return receipt, err
	}
	if err = requireGroupRoomContext(contextValue); err != nil {
		return receipt, err
	}
	if err = s.AuthorizeAssignmentTarget(ctx, orchestrationsvc.AssignmentTargetRequest{
		OwnerUserID:    delivery.OwnerUserID,
		SessionKey:     delivery.SessionKey,
		ExecutionID:    delivery.Binding.ExecutionID,
		RoomID:         delivery.RoomID,
		ConversationID: delivery.ConversationID,
		ActorAgentID:   delivery.SourceAgentID,
		TargetAgentID:  delivery.TargetAgentID,
		Strategy:       protocol.AssignmentStrategyRoomMember,
	}); err != nil {
		return receipt, err
	}
	handoffID := "execution_review_dispatch_" + delivery.Binding.ReviewDispatchID
	content := renderExecutionReviewDispatchInstruction(delivery)
	accepted, existingReceipt, err := s.ensureExecutionReviewDispatchHandoff(
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
		CoordinatorAgentID: strings.TrimSpace(delivery.TargetAgentID),
		RoomType:           contextValue.Room.RoomType,
		Context:            contextValue,
		RoundID:            handoffID,
		RootRoundID:        handoffID,
		OwnerUserID:        contextValue.Room.OwnerUserID,
	}
	if err = s.authorizeManagedExecutionReviewTarget(
		ctx,
		parentRound,
		delivery.TargetAgentID,
		&delivery.Binding,
	); err != nil {
		return receipt, err
	}
	return s.enqueueExecutionReviewDispatch(
		ctx,
		contextValue,
		parentRound,
		delivery,
		handoffID,
		content,
	)
}

func (s *Service) enqueueExecutionReviewDispatch(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	parentRound *activeRoomRound,
	delivery orchestrationsvc.ExecutionReviewDispatchDelivery,
	handoffID string,
	content string,
) (orchestrationsvc.ExecutionReviewDispatchReceipt, error) {
	var receipt orchestrationsvc.ExecutionReviewDispatchReceipt
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
		return receipt, errors.New(
			"Execution Review Dispatch target has no durable queue location",
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
		ReviewBinding:   cloneExecutionReviewBinding(&delivery.Binding),
	}
	items, inserted, err := s.inputQueue.EnqueueBounded(
		location.Location,
		item,
		0,
	)
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
		if !executionReviewBindingEqual(item.ReviewBinding, &delivery.Binding) {
			return receipt, errors.New(
				"Execution Review Dispatch queue identity conflicts with durable Room state",
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
			"广播 Execution Review Dispatch 队列快照失败",
			"review_dispatch_id",
			delivery.Binding.ReviewDispatchID,
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
	return orchestrationsvc.ExecutionReviewDispatchReceipt{
		HandoffID:   handoffID,
		QueueItemID: item.ID,
	}, nil
}

func (s *Service) ensureExecutionReviewDispatchHandoff(
	delivery orchestrationsvc.ExecutionReviewDispatchDelivery,
	handoffID string,
	content string,
) (
	bool,
	orchestrationsvc.ExecutionReviewDispatchReceipt,
	error,
) {
	var receipt orchestrationsvc.ExecutionReviewDispatchReceipt
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
			ReviewBinding:      &binding,
		},
	)
	if err != nil {
		return false, receipt, err
	}
	if !executionReviewBindingEqual(handoff.ReviewBinding, &binding) ||
		handoff.WorkBinding != nil ||
		strings.TrimSpace(handoff.SourceAgentID) !=
			strings.TrimSpace(delivery.SourceAgentID) ||
		strings.TrimSpace(handoff.TargetAgentID) !=
			strings.TrimSpace(delivery.TargetAgentID) {
		return false, receipt, errors.New(
			"Execution Review Dispatch handoff identity conflicts with durable Room state",
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
		return true, orchestrationsvc.ExecutionReviewDispatchReceipt{
			HandoffID:   handoffID,
			QueueItemID: strings.TrimSpace(handoff.QueueItemID),
		}, nil
	}
	return false, receipt, nil
}

func (s *Service) authorizeManagedExecutionReviewTarget(
	ctx context.Context,
	roundValue *activeRoomRound,
	targetAgentID string,
	binding *protocol.ExecutionReviewBinding,
) error {
	if s == nil || s.executionContext == nil || roundValue == nil {
		return errors.New("managed Execution review admission is unavailable")
	}
	authorizer, ok := s.executionContext.(executionReviewTargetAuthorizer)
	if !ok {
		return errors.New("managed Execution review target admission is unavailable")
	}
	return authorizer.AuthorizeRoomReviewReturn(
		ctx,
		orchestrationsvc.ActorContext{
			OwnerUserID:    roundValue.OwnerUserID,
			SessionKey:     roundValue.SessionKey,
			ExecutionID:    executionIDFromReviewBinding(binding),
			ReviewBinding:  cloneExecutionReviewBinding(binding),
			AgentID:        strings.TrimSpace(targetAgentID),
			ActorKind:      protocol.ExecutionActorAgent,
			ScopeKind:      protocol.ExecutionScopeRoom,
			RoomID:         roundValue.RoomID,
			ConversationID: roundValue.ConversationID,
			RootRoundID:    roomRootRoundID(roundValue),
		},
		binding,
	)
}

func validateExecutionReviewDispatchDelivery(
	delivery orchestrationsvc.ExecutionReviewDispatchDelivery,
) error {
	values := map[string]string{
		"owner_user_id":      delivery.OwnerUserID,
		"session_key":        delivery.SessionKey,
		"room_id":            delivery.RoomID,
		"conversation_id":    delivery.ConversationID,
		"source_agent_id":    delivery.SourceAgentID,
		"target_agent_id":    delivery.TargetAgentID,
		"instruction":        delivery.Instruction,
		"execution_id":       delivery.Binding.ExecutionID,
		"plan_id":            delivery.Binding.PlanID,
		"work_item_id":       delivery.Binding.WorkItemID,
		"spec_id":            delivery.Binding.SpecID,
		"assignment_id":      delivery.Binding.AssignmentID,
		"submission_id":      delivery.Binding.SubmissionID,
		"review_dispatch_id": delivery.Binding.ReviewDispatchID,
		"binding_target_id":  delivery.Binding.TargetAgentID,
	}
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if strings.TrimSpace(delivery.TargetAgentID) !=
		strings.TrimSpace(delivery.Binding.TargetAgentID) {
		return errors.New("review delivery target conflicts with trusted binding")
	}
	return nil
}

func renderExecutionReviewDispatchInstruction(
	delivery orchestrationsvc.ExecutionReviewDispatchDelivery,
) string {
	binding := delivery.Binding
	return fmt.Sprintf(
		"[Nexus structured review return]\n"+
			"execution_id: %s\nplan_id: %s\nwork_item_id: %s\nspec_id: %s\n"+
			"assignment_id: %s\nsubmission_id: %s\nreview_dispatch_id: %s\n"+
			"target_agent_id: %s\n\n%s\n\n"+
			"Inspect pending_reviews in nexus_execution_context and call review_work. "+
			"Do not infer review authority from this text.",
		binding.ExecutionID,
		binding.PlanID,
		binding.WorkItemID,
		binding.SpecID,
		binding.AssignmentID,
		binding.SubmissionID,
		binding.ReviewDispatchID,
		binding.TargetAgentID,
		strings.TrimSpace(delivery.Instruction),
	)
}

func executionIDFromReviewBinding(
	binding *protocol.ExecutionReviewBinding,
) string {
	if binding == nil {
		return ""
	}
	return strings.TrimSpace(binding.ExecutionID)
}

func executionReviewBindingEqual(
	left *protocol.ExecutionReviewBinding,
	right *protocol.ExecutionReviewBinding,
) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Normalized() == right.Normalized()
}
