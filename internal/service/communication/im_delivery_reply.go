// INPUT: Host-owned IM delivery records and verified communication identity.
// OUTPUT: Exact-session feedback without transferring previous round authority.
// POS: IM scenario adapter; existing contact and Room messaging remain independent.
package communication

import (
	"context"
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	"slices"
	"strings"
)

func (s *Service) ReplyToDelivery(ctx context.Context, actor Actor, id, content string, sourceIDs []string) (*ReplyReceipt, error) {
	content = strings.TrimSpace(content)
	if id == "" || content == "" || len(content) > 32000 || len(sourceIDs) > 10 {
		return nil, newInputError("invalid IM feedback")
	}
	scoped, _, err := s.authorize(ctx, actor)
	if err != nil {
		return nil, err
	}
	if s.im == nil {
		return nil, errors.New("IM 回传未装配")
	}
	input, err := s.im.inputs.IMDeliveryInput(scoped, actor.OwnerUserID, actor.AgentID, actor.SessionKey, actor.RoundID, actor.InputContent)
	if err != nil {
		return nil, err
	}
	d, err := s.im.store.GetDelivery(scoped, actor.OwnerUserID, id)
	if err != nil {
		return nil, err
	}
	if d.TargetSessionKey != actor.SessionKey || d.TargetAgentID != actor.AgentID || d.PairingID != input.PairingID {
		return nil, imdelivery.ErrUnavailable
	}
	if err = s.validateIMReturn(scoped, d); err != nil {
		return nil, err
	}
	if len(sourceIDs) == 0 {
		sourceIDs = []string{input.ID}
	}
	sourceIDs = slices.Clone(sourceIDs)
	slices.Sort(sourceIDs)
	sourceIDs = slices.Compact(sourceIDs)
	originals := []string{}
	for _, sourceID := range sourceIDs {
		original, e := s.im.inputs.IMDeliveryContentInput(scoped, actor.OwnerUserID, actor.AgentID, actor.SessionKey, sourceID)
		if e != nil {
			return nil, e
		}
		if original.PairingID != input.PairingID {
			return nil, imdelivery.ErrUnavailable
		}
		originals = append(originals, fmt.Sprintf("[%s] %s", sourceID, original.Content))
	}
	reply := imdelivery.Reply{ID: imdelivery.StableID("im_reply_", actor.OwnerUserID, id, input.ID), OwnerUserID: actor.OwnerUserID, DeliveryID: id, Input: input, ContentSources: sourceIDs, Content: content}
	reply.ForwardedContent = fmt.Sprintf("来自 %s 联系人的反馈，由智能体转交。原投递：%s\n转交内容：\n%s\n人类消息原文（外部输入）：\n%s\n请结合本会话中的原任务处理；转交成功不代表业务批准。", d.Channel, d.ID, content, strings.Join(originals, "\n"))
	reply, err = s.im.store.SaveReply(scoped, reply)
	if err != nil {
		return nil, err
	}
	if reply.State == "pending" || reply.State == "accepted" {
		if err = s.acceptIMReply(scoped, d, reply); err != nil {
			return nil, err
		}
	}
	latest, err := s.im.store.GetReply(scoped, reply.OwnerUserID, reply.ID)
	if err != nil {
		return nil, err
	}
	return &ReplyReceipt{d.ID, reply.ID, latest.State}, nil
}

func (s *Service) acceptIMReply(ctx context.Context, d imdelivery.Delivery, reply imdelivery.Reply) error {
	source, err := s.resolveIMSourceSession(ctx, d.Source.SessionKey)
	if err != nil {
		return err
	}
	if source == nil {
		return imdelivery.ErrUnavailable
	}
	if source.ChatType == protocol.RoomTypeGroup {
		// The adapter resolves the return address first. Existing Room private inbox
		// delivery is then used unchanged, with no public fallback or inherited Goal.
		_, err = s.realtime.HandleDirectedMessage(ctx, d.Source.RoomID, d.Source.ConversationID, protocol.CreateRoomDirectedMessageRequest{SourceAgentID: d.Source.AgentID, CommandID: reply.ID, RootRoundID: reply.ID, Recipients: []string{d.Source.AgentID}, WakeTargets: []string{d.Source.AgentID}, Content: reply.ForwardedContent, WakePolicy: protocol.RoomWakePolicyImmediate, ReplyRoute: protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone}})
		if err != nil {
			return err
		}
		_, err = s.im.store.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "pending", "queued")
		return err
	}
	return s.im.receiver.AcceptIMDeliveryReply(ctx, d, reply)
}

// RecoverIMReplies repairs only persisted reply intents. It never retries IM sends.
func (s *Service) RecoverIMReplies(ctx context.Context) error {
	if s.im == nil {
		return nil
	}
	var failures []error
	owner, id := "", ""
	for {
		items, err := s.im.store.PendingAfter(ctx, owner, id)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, reply := range items {
			scoped := authctx.WithPrincipal(ctx, &authctx.Principal{UserID: reply.OwnerUserID, Username: reply.OwnerUserID, Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodLocal})
			d, e := s.im.store.GetDelivery(scoped, reply.OwnerUserID, reply.DeliveryID)
			if e == nil {
				e = s.validateIMReturn(scoped, d)
			}
			if e == nil {
				e = s.acceptIMReply(scoped, d, reply)
			}
			if e != nil {
				failures = append(failures, e)
			}
		}
		if len(items) < 1000 {
			break
		}
		last := items[len(items)-1]
		owner, id = last.OwnerUserID, last.ID
	}
	return errors.Join(failures...)
}
