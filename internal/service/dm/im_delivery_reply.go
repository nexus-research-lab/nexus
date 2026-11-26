// INPUT: Host-persisted IM feedback and its exact original DM Session.
// OUTPUT: Idempotent durable admission (main window inbox or JSONL queue) and one-shot dispatch with provenance.
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
	inbox, mainWindow, err := s.mainInboxForExecution(ctx, d.Source.SessionKey, d.Source.AgentID)
	if err != nil {
		return err
	}
	if mainWindow {
		return s.acceptMainInboxIMReply(ctx, inbox, reply)
	}
	session, location, err := s.resolveInputQueueLocation(ctx, d.Source.SessionKey, d.Source.AgentID)
	if err != nil {
		return err
	}
	unlock, err := s.dispatchLocks.lock(ctx, session)
	if err != nil {
		return err
	}
	result, err := s.InputQueue.EnqueueIdempotent(location, protocol.InputQueueItem{
		ID: reply.ID, Scope: protocol.InputQueueScopeDM, SessionKey: session, AgentID: d.Source.AgentID,
		ClientMessageID: reply.ID, SourceMessageID: "im_feedback_" + reply.ID, Source: protocol.InputQueueSourceIMDeliveryReply,
		Content: reply.ForwardedContent, DeliveryPolicy: protocol.ChatDeliveryPolicyQueue, OwnerUserID: reply.OwnerUserID,
	}, reply.ID)
	unlock()
	if err != nil {
		return err
	}
	if _, err = s.imReplies.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "pending", "accepted"); err != nil {
		return err
	}
	if _, pending := inputQueueItemByID(result.Items, result.Item.ID); pending {
		s.broadcastInputQueueSnapshot(ctx, session, result.Items)
		s.StartSessionBackgroundTask(session, reply.OwnerUserID, func(taskCtx context.Context) {
			s.dispatchNextInputQueueItemAtLocation(taskCtx, session, d.Source.AgentID, location)
		})
	} else {
		// A durable enqueue with no remaining item and no dispatch claim is an
		// uncertain crash/cancellation window; never infer it is safe to run again.
		_, err = s.imReplies.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "accepted", "needs_attention")
	}
	return err
}

// imReplyDispatch 是一次 IM 回执起轮所需的全部身份；JSONL 队列项与主窗口 inbox 事件都还原成它。
type imReplyDispatch struct {
	sessionKey  string
	agentID     string
	ownerUserID string
	replyID     string
	content     string
}

// imReplyDispatchFromQueueItem 校验 JSONL 队列项仍是宿主写入的回执形状。
func imReplyDispatchFromQueueItem(session string, item protocol.InputQueueItem) (imReplyDispatch, error) {
	if item.ID != item.ClientMessageID || item.SourceMessageID != "im_feedback_"+item.ID || len(item.Attachments) != 0 {
		return imReplyDispatch{}, imdelivery.ErrConflict
	}
	return imReplyDispatch{sessionKey: session, agentID: item.AgentID, ownerUserID: item.OwnerUserID, replyID: item.ID, content: item.Content}, nil
}

// dispatchIMDeliveryReply 复核回执后起一轮；started=false 表示回执已被别处认领，本次什么都没做。
func (s *Service) dispatchIMDeliveryReply(ctx context.Context, reply imReplyDispatch) (bool, error) {
	if s.imReplies == nil || s.imReplyValidate == nil {
		return false, errors.New("IM reply dispatch unavailable")
	}
	if _, err := s.imReplies.VerifyReply(ctx, reply.ownerUserID, reply.replyID, reply.sessionKey, reply.agentID, reply.content); err != nil {
		return false, err
	}
	if err := s.imReplyValidate(ctx, reply.ownerUserID, reply.replyID); err != nil {
		return false, err
	}
	var policy *protocol.RuntimeToolPolicy
	saved, err := s.imReplies.GetReply(ctx, reply.ownerUserID, reply.replyID)
	if err != nil {
		return false, err
	}
	delivery, err := s.imReplies.GetDelivery(ctx, reply.ownerUserID, saved.DeliveryID)
	if err != nil {
		return false, err
	}
	if delivery.Source.Kind == "automation" {
		if s.imAutomationPolicy == nil {
			return false, errors.New("automation feedback admission unavailable")
		}
		policy, err = s.imAutomationPolicy(ctx, delivery.Source)
		if err != nil {
			return false, err
		}
	}
	claimed, err := s.imReplies.TransitionReply(ctx, reply.ownerUserID, reply.replyID, "accepted", "started")
	if err != nil {
		return false, err
	}
	if !claimed {
		claimed, err = s.imReplies.TransitionReply(ctx, reply.ownerUserID, reply.replyID, "pending", "started")
		if err != nil {
			return false, err
		}
	}
	if !claimed {
		return false, nil
	}
	request := Request{SessionKey: reply.sessionKey, AgentID: reply.agentID, Content: reply.content, ClientMessageID: reply.replyID, RoundID: reply.replyID, UserMessageID: "im_feedback_" + reply.replyID, DeliveryPolicy: protocol.ChatDeliveryPolicyQueue, BroadcastUserMessage: true, ExecutionOrigin: "im_delivery_reply"}
	request.RuntimeToolPolicy = policy
	request.InputOptions.Metadata = map[string]string{"source": "im_delivery_reply", "reply_id": reply.replyID}
	err = s.handleChat(ctx, request, chatExecutionInline)
	if err != nil {
		_, persistErr := s.imReplies.TransitionReply(ctx, reply.ownerUserID, reply.replyID, "started", "needs_attention")
		return false, errors.Join(err, persistErr)
	}
	return true, nil
}

func (s *Service) SetIMAutomationPolicy(policy func(context.Context, imdelivery.Source) (*protocol.RuntimeToolPolicy, error)) {
	s.imAutomationPolicy = policy
}
