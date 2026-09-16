// INPUT: Host-persisted IM feedback and its exact original DM Session.
// OUTPUT: Idempotent durable queue admission and one-shot dispatch with provenance.
// POS: DM adapter for IM replies; does not confer local-user configuration authority.
package dm

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

func (s *Service) SetIMDeliveryStore(store *imdelivery.Repository, validate func(context.Context, string, string) error) {
	s.imReplies = store
	s.imReplyValidate = validate
}

func (s *Service) AcceptIMDeliveryReply(ctx context.Context, d imdelivery.Delivery, reply imdelivery.Reply) error {
	if s.imReplies == nil || s.imReplyValidate == nil {
		return errors.New("IM reply admission unavailable")
	}
	if authctx.OwnerUserID(ctx) != reply.OwnerUserID || d.OwnerUserID != reply.OwnerUserID {
		return imdelivery.ErrUnavailable
	}
	if _, err := s.imReplies.VerifyReply(ctx, reply.OwnerUserID, reply.ID, d.Source.SessionKey, d.Source.AgentID, reply.ForwardedContent); err != nil {
		return err
	}
	if err := s.imReplyValidate(ctx, reply.OwnerUserID, reply.ID); err != nil {
		return err
	}
	session, location, err := s.resolveInputQueueLocation(ctx, d.Source.SessionKey, d.Source.AgentID)
	if err != nil {
		return err
	}
	if err = s.inputQueueDispatchMu.LockContext(ctx); err != nil {
		return err
	}
	result, err := s.inputQueue.EnqueueIdempotent(location, protocol.InputQueueItem{
		ID: reply.ID, Scope: protocol.InputQueueScopeDM, SessionKey: session, AgentID: d.Source.AgentID,
		ClientMessageID: reply.ID, SourceMessageID: "im_feedback_" + reply.ID, Source: protocol.InputQueueSourceIMDeliveryReply,
		Content: reply.ForwardedContent, DeliveryPolicy: protocol.ChatDeliveryPolicyQueue, OwnerUserID: reply.OwnerUserID,
	}, reply.ID)
	s.inputQueueDispatchMu.Unlock()
	if err != nil {
		return err
	}
	if _, err = s.imReplies.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "pending", "accepted"); err != nil {
		return err
	}
	if _, pending := inputQueueItemByID(result.Items, result.Item.ID); pending {
		s.broadcastInputQueueSnapshot(ctx, session, result.Items)
		s.startSessionBackgroundTask(session, reply.OwnerUserID, func(taskCtx context.Context) {
			s.dispatchNextInputQueueItemAtLocation(taskCtx, session, d.Source.AgentID, location)
		})
	} else {
		// A durable enqueue with no remaining item and no dispatch claim is an
		// uncertain crash/cancellation window; never infer it is safe to run again.
		_, err = s.imReplies.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "accepted", "needs_attention")
	}
	return err
}

func (s *Service) dispatchIMDeliveryReply(ctx context.Context, session string, item protocol.InputQueueItem) error {
	if s.imReplies == nil || s.imReplyValidate == nil {
		return errors.New("IM reply dispatch unavailable")
	}
	if item.ID != item.ClientMessageID || item.SourceMessageID != "im_feedback_"+item.ID || len(item.Attachments) != 0 {
		return imdelivery.ErrConflict
	}
	if _, err := s.imReplies.VerifyReply(ctx, item.OwnerUserID, item.ID, session, item.AgentID, item.Content); err != nil {
		return err
	}
	if err := s.imReplyValidate(ctx, item.OwnerUserID, item.ID); err != nil {
		return err
	}
	var policy *protocol.RuntimeToolPolicy
	saved, err := s.imReplies.GetReply(ctx, item.OwnerUserID, item.ID)
	if err != nil {
		return err
	}
	delivery, err := s.imReplies.GetDelivery(ctx, item.OwnerUserID, saved.DeliveryID)
	if err != nil {
		return err
	}
	if delivery.Source.Kind == "automation" {
		if s.imAutomationPolicy == nil {
			return errors.New("automation feedback admission unavailable")
		}
		policy, err = s.imAutomationPolicy(ctx, delivery.Source)
		if err != nil {
			return err
		}
	}
	claimed, err := s.imReplies.TransitionReply(ctx, item.OwnerUserID, item.ID, "accepted", "started")
	if err != nil {
		return err
	}
	if !claimed {
		claimed, err = s.imReplies.TransitionReply(ctx, item.OwnerUserID, item.ID, "pending", "started")
		if err != nil {
			return err
		}
	}
	if !claimed {
		return nil
	}
	request := Request{SessionKey: session, AgentID: item.AgentID, Content: item.Content, ClientMessageID: item.ClientMessageID, RoundID: item.ID, UserMessageID: item.SourceMessageID, DeliveryPolicy: protocol.ChatDeliveryPolicyQueue, BroadcastUserMessage: true, ExecutionOrigin: "im_delivery_reply"}
	request.RuntimeToolPolicy = policy
	request.InputOptions.Metadata = map[string]string{"source": "im_delivery_reply", "reply_id": item.ID}
	err = s.handleChat(ctx, request, chatExecutionInline)
	if err != nil {
		_, persistErr := s.imReplies.TransitionReply(ctx, item.OwnerUserID, item.ID, "started", "needs_attention")
		return errors.Join(err, persistErr)
	}
	return nil
}

func (s *Service) SetIMAutomationPolicy(policy func(context.Context, imdelivery.Source) (*protocol.RuntimeToolPolicy, error)) {
	s.imAutomationPolicy = policy
}
