package server

import (
	"context"
	"errors"

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
) error {
	if d.router == nil {
		return errors.New("channel router is not configured")
	}
	_, err := d.router.DeliverText(ctx, agentID, text, channels.DeliveryTarget{
		Mode:       target.Mode,
		Channel:    target.Channel,
		To:         target.To,
		AccountID:  target.AccountID,
		ThreadID:   target.ThreadID,
		SessionKey: target.SessionKey,
	})
	return err
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
