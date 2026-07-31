// INPUT: Structured DM session identity plus an optional exact runtime round identity.
// OUTPUT: Exact-round cancellation without successor damage, or explicit whole-session cancellation.
// POS: DM cancellation boundary; a stale round stop must never broaden into a session stop.
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

var (
	ErrTargetDMRoundNotRunning          = errors.New("target DM round not found or already ended")
	ErrExactDMRoundInterruptUnsupported = errors.New("exact DM round interrupt is unsupported")
)

// HandleInterrupt 处理中断请求。带 round_id 时只取消该物理轮次；只有明确
// 省略 round_id 的调用才表示停止整个 DM session。
func (s *Service) HandleInterrupt(ctx context.Context, request InterruptRequest) error {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return err
	}
	if roundID := strings.TrimSpace(request.RoundID); roundID != "" {
		return s.interruptExactRound(ctx, sessionKey, roundID)
	}
	return s.interruptSession(ctx, sessionKey, messagepkg.InterruptWithoutMessage)
}

func (s *Service) interruptExactRound(ctx context.Context, sessionKey string, roundID string) error {
	result, err := s.runtime.InterruptRound(
		ctx,
		sessionKey,
		roundID,
		messagepkg.InterruptWithoutMessage,
	)
	if err != nil {
		return err
	}
	switch result.Outcome {
	case runtimectx.ExactRoundAlreadyEnded:
		return ErrTargetDMRoundNotRunning
	case runtimectx.ExactRoundInterruptUnsupported:
		if detail := strings.TrimSpace(result.Detail); detail != "" {
			return fmt.Errorf("%w: %s", ErrExactDMRoundInterruptUnsupported, detail)
		}
		return ErrExactDMRoundInterruptUnsupported
	case runtimectx.ExactRoundProviderInterrupted, runtimectx.ExactRoundLocalCancelled:
		// Permission requests are keyed only by session. Cancelling them while a
		// successor is still running would widen an exact-round stop.
		if len(s.runtime.GetRunningRoundIDs(sessionKey)) == 0 {
			s.permission.CancelRequestsForSession(sessionKey, "")
		}
	default:
		return fmt.Errorf("%w: unknown runtime outcome %q", ErrExactDMRoundInterruptUnsupported, result.Outcome)
	}
	if closeErr := s.refreshSessionMetaRuntimeStateByKey(ctx, sessionKey); closeErr != nil {
		s.loggerFor(ctx).Warn("DM 精确中断后刷新 session meta 失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"err", closeErr,
		)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	return nil
}

func (s *Service) interruptSession(ctx context.Context, sessionKey string, resultText string) error {
	displayResultText := resultText
	if displayResultText == messagepkg.InterruptWithoutMessage {
		displayResultText = ""
	}
	roundIDs, err := s.runtime.InterruptSession(ctx, sessionKey, resultText)
	if err != nil {
		if len(roundIDs) == 0 {
			return err
		}
		s.loggerFor(ctx).Warn("DM 中断运行态失败，按失效进程清理",
			"session_key", sessionKey,
			"round_ids", roundIDs,
			"err", err,
		)
		if closeErr := s.runtime.CloseSession(context.Background(), sessionKey); closeErr != nil {
			s.loggerFor(ctx).Warn("DM 清理失效运行态 client 失败",
				"session_key", sessionKey,
				"err", closeErr,
			)
		}
		s.permission.CancelRequestsForSession(sessionKey, displayResultText)
		if closeErr := s.refreshSessionMetaRuntimeStateByKey(ctx, sessionKey); closeErr != nil {
			s.loggerFor(ctx).Warn("DM 中断失败后刷新 session meta 失败",
				"session_key", sessionKey,
				"err", closeErr,
			)
		}
		s.broadcastSessionStatus(ctx, sessionKey)
		return nil
	}
	if len(roundIDs) == 0 {
		if closeErr := s.refreshSessionMetaRuntimeStateByKey(ctx, sessionKey); closeErr != nil {
			s.loggerFor(ctx).Warn("DM 中断空闲会话后刷新 session meta 失败",
				"session_key", sessionKey,
				"err", closeErr,
			)
		}
		s.broadcastSessionStatus(ctx, sessionKey)
		return nil
	}
	s.loggerFor(ctx).Warn("中断 DM 会话运行轮次",
		"session_key", sessionKey,
		"round_count", len(roundIDs),
		"reason", displayResultText,
	)
	s.permission.CancelRequestsForSession(sessionKey, displayResultText)
	if closeErr := s.refreshSessionMetaRuntimeStateByKey(ctx, sessionKey); closeErr != nil {
		s.loggerFor(ctx).Warn("DM 中断后刷新 session meta 失败",
			"session_key", sessionKey,
			"err", closeErr,
		)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	return nil
}
