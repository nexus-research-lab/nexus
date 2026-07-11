package dm

import (
	"context"
	"slices"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels/typingloop"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const externalReplyTimeout = 45 * time.Second
const externalTypingTimeout = typingloop.DefaultCallTimeout
const externalTypingStartDelay = typingloop.DefaultStartDelay
const externalTypingKeepaliveInterval = typingloop.DefaultKeepaliveInterval

func (r *roundRunner) startExternalReplyTyping(ctx context.Context) func() {
	agentID, target, ok := r.externalReplyTypingTarget()
	if !ok {
		return func() {}
	}

	return typingloop.Start(ctx, func(callCtx context.Context, active bool) error {
		return r.service.replies.SetExternalTyping(callCtx, agentID, target, active)
	}, typingloop.LoopOptions{
		StartDelay:        externalTypingStartDelay,
		KeepaliveInterval: externalTypingKeepaliveInterval,
		CallTimeout:       externalTypingTimeout,
		OnError: func(active bool, err error) {
			r.logExternalTypingError(agentID, target, active, err)
		},
	})
}

func (r *roundRunner) deliverExternalAssistantReply(ctx context.Context, assistant protocol.Message) {
	agentID, target, ok := r.externalReplyTypingTarget()
	if !ok {
		return
	}
	text := messageutil.ExtractAssistantDisplayText(assistant)
	if strings.TrimSpace(text) == "" {
		return
	}

	deliverCtx, cancel := context.WithTimeout(ctx, externalReplyTimeout)
	defer cancel()
	result, err := r.service.replies.DeliverExternalReply(deliverCtx, agentID, text, target)
	if err != nil {
		r.service.loggerFor(context.Background()).Error("DM assistant 外部通道回复投递失败",
			"session_key", r.sessionKey,
			"agent_id", agentID,
			"round_id", r.roundID,
			"channel", target.Channel,
			"to", target.To,
			"thread_id", target.ThreadID,
			"err", err,
		)
		return
	}
	r.persistExternalReplyReceipt(assistant, result)
	r.service.loggerFor(context.Background()).Info("DM assistant 外部通道回复已投递",
		"session_key", r.sessionKey,
		"agent_id", agentID,
		"round_id", r.roundID,
		"channel", target.Channel,
		"to", target.To,
		"thread_id", target.ThreadID,
		"primary_platform_message_id", result.PrimaryPlatformMessageID,
		"platform_message_ids", result.PlatformMessageIDs,
		"chars", len([]rune(strings.TrimSpace(text))),
	)
}

func (r *roundRunner) persistExternalReplyReceipt(assistant protocol.Message, result ExternalReplyResult) {
	if r == nil || r.service == nil || r.service.history == nil {
		return
	}
	if strings.TrimSpace(r.workspacePath) == "" || strings.TrimSpace(r.session.SessionKey) == "" {
		return
	}

	receipt := workspacestore.ExternalDeliveryReceipt{
		RoundID:                  r.roundID,
		MessageID:                dmdomain.NormalizeString(assistant["message_id"]),
		Channel:                  result.Channel,
		Target:                   result.To,
		ThreadID:                 result.ThreadID,
		PrimaryPlatformMessageID: result.PrimaryPlatformMessageID,
		PlatformMessageIDs:       slices.Clone(result.PlatformMessageIDs),
		Timestamp:                time.Now().UTC(),
	}
	if err := r.service.history.AppendExternalDeliveryReceipt(r.workspacePath, r.session.SessionKey, receipt); err != nil {
		r.service.loggerFor(context.Background()).Warn("DM assistant 外部通道回执持久化失败",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
			"message_id", receipt.MessageID,
			"channel", result.Channel,
			"to", result.To,
			"err", err,
		)
	}
}

func (r *roundRunner) externalReplyTypingTarget() (string, ExternalReplyTarget, bool) {
	if r == nil || r.service == nil || r.service.replies == nil || r.externalReplyTarget == nil {
		return "", ExternalReplyTarget{}, false
	}
	if r.internal || !isExternalReplySessionKey(r.sessionKey) {
		return "", ExternalReplyTarget{}, false
	}
	agentID := ""
	if r.agent != nil {
		agentID = r.agent.AgentID
	}
	if strings.TrimSpace(agentID) == "" {
		return "", ExternalReplyTarget{}, false
	}
	target := *r.externalReplyTarget
	if strings.TrimSpace(target.Mode) == "" {
		target.Mode = "explicit"
	}
	return agentID, target, true
}

func (r *roundRunner) logExternalTypingError(agentID string, target ExternalReplyTarget, active bool, err error) {
	r.service.loggerFor(context.Background()).Warn("DM assistant 外部通道 typing 状态投递失败",
		"session_key", r.sessionKey,
		"agent_id", agentID,
		"round_id", r.roundID,
		"channel", target.Channel,
		"to", target.To,
		"active", active,
		"err", err,
	)
}

func isExternalReplySessionKey(sessionKey string) bool {
	parsed := protocol.ParseSessionKey(sessionKey)
	if !parsed.IsStructured || parsed.Kind != protocol.SessionKeyKindAgent {
		return false
	}
	switch protocol.NormalizeStoredChannelType(parsed.Channel) {
	case "", protocol.SessionChannelWebSocket, protocol.SessionChannelInternalSegment:
		return false
	default:
		return true
	}
}
