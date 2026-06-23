package server

import (
	"context"
	"errors"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
)

type dmExternalReplyDispatcher struct {
	router *channels.Router
}

func (d dmExternalReplyDispatcher) DeliverExternalReply(
	ctx context.Context,
	agentID string,
	text string,
	target dmsvc.ExternalReplyTarget,
) (dmsvc.ExternalReplyResult, error) {
	if d.router == nil {
		return dmsvc.ExternalReplyResult{}, errors.New("channel router is not configured")
	}
	result, err := d.router.DeliverMessage(ctx, agentID, text, channels.DeliveryTarget{
		Mode:       target.Mode,
		Channel:    target.Channel,
		To:         target.To,
		AccountID:  target.AccountID,
		ThreadID:   target.ThreadID,
		SessionKey: target.SessionKey,
	})
	if err != nil {
		return dmsvc.ExternalReplyResult{}, err
	}
	reply := dmsvc.ExternalReplyResult{
		Channel:  result.Target.Channel,
		To:       result.Target.To,
		ThreadID: result.Target.ThreadID,
	}
	if result.Receipt != nil {
		reply.PrimaryPlatformMessageID = result.Receipt.PrimaryPlatformMessageID
		reply.PlatformMessageIDs = slices.Clone(result.Receipt.PlatformMessageIDs)
	}
	return reply, nil
}

func (d dmExternalReplyDispatcher) SetExternalTyping(
	ctx context.Context,
	agentID string,
	target dmsvc.ExternalReplyTarget,
	active bool,
) error {
	if d.router == nil {
		return errors.New("channel router is not configured")
	}
	return d.router.SetTyping(ctx, agentID, channels.DeliveryTarget{
		Mode:       target.Mode,
		Channel:    target.Channel,
		To:         target.To,
		AccountID:  target.AccountID,
		ThreadID:   target.ThreadID,
		SessionKey: target.SessionKey,
	}, active)
}
