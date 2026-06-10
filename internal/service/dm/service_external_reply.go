package dm

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const externalReplyTimeout = 45 * time.Second
const externalTypingTimeout = 10 * time.Second
const externalTypingStartDelay = 800 * time.Millisecond
const externalTypingKeepaliveInterval = 5 * time.Second

func (r *roundRunner) startExternalReplyTyping(ctx context.Context) func() {
	agentID, target, ok := r.externalReplyTypingTarget()
	if !ok {
		return func() {}
	}

	typingCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	activeStarted := atomic.Bool{}
	go func() {
		defer close(done)
		timer := time.NewTimer(externalTypingStartDelay)
		defer timer.Stop()
		select {
		case <-typingCtx.Done():
			return
		case <-timer.C:
		}
		activeStarted.Store(true)
		r.setExternalTyping(typingCtx, agentID, target, true)
		ticker := time.NewTicker(externalTypingKeepaliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				r.setExternalTyping(typingCtx, agentID, target, true)
			}
		}
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		if !activeStarted.Load() {
			return
		}
		cancelCtx, cancelStop := context.WithTimeout(context.Background(), externalTypingTimeout)
		defer cancelStop()
		r.setExternalTyping(cancelCtx, agentID, target, false)
	}
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
	if err := r.service.replies.DeliverExternalReply(deliverCtx, agentID, text, target); err != nil {
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
	r.service.loggerFor(context.Background()).Info("DM assistant 外部通道回复已投递",
		"session_key", r.sessionKey,
		"agent_id", agentID,
		"round_id", r.roundID,
		"channel", target.Channel,
		"to", target.To,
		"thread_id", target.ThreadID,
		"chars", len([]rune(strings.TrimSpace(text))),
	)
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

func (r *roundRunner) setExternalTyping(ctx context.Context, agentID string, target ExternalReplyTarget, active bool) {
	callCtx, cancel := context.WithTimeout(ctx, externalTypingTimeout)
	defer cancel()
	if err := r.service.replies.SetExternalTyping(callCtx, agentID, target, active); err != nil {
		if active && ctx.Err() != nil {
			return
		}
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
