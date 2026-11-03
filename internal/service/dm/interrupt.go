// INPUT: Structured DM session identity, an optional exact runtime round identity and whether the user pressed stop.
// OUTPUT: Exact-round or whole-session cancellation; only a user stop writes the queue pause gate, host control stops carry their own reason.
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
	reason := messagepkg.InterruptHostControlWithoutMessage
	if request.UserStop {
		reason = messagepkg.InterruptWithoutMessage
		if err := s.pauseInputQueueForUserStop(ctx, sessionKey, request.RoundID); err != nil {
			return err
		}
	}
	if roundID := strings.TrimSpace(request.RoundID); roundID != "" {
		return s.interruptExactRound(ctx, sessionKey, roundID, reason)
	}
	return s.interruptSession(ctx, sessionKey, reason)
}

// pauseInputQueueForUserStop serializes the durable stop gate with dispatch
// admission, but releases it before waiting for a running round to drain.
func (s *Service) pauseInputQueueForUserStop(ctx context.Context, sessionKey string, roundID string) error {
	if err := s.inputQueueDispatchMu.LockContext(ctx); err != nil {
		return err
	}
	defer s.inputQueueDispatchMu.Unlock()
	targetActive := roundID == ""
	for _, id := range s.Runtime.GetRunningRoundIDs(sessionKey) {
		if id == roundID {
			targetActive = true
		}
	}
	if !targetActive {
		return nil
	}
	_, location, err := s.resolveInputQueueLocation(ctx, sessionKey, "")
	if err == nil {
		err = s.InputQueue.SetDispatchPaused(location, true)
	}
	if err != nil {
		return err
	}
	if items, snapshotErr := s.InputQueue.Snapshot(location); snapshotErr == nil {
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
	}
	return nil
}

func (s *Service) interruptExactRound(ctx context.Context, sessionKey string, roundID string, reason string) error {
	result, err := s.Runtime.InterruptRound(
		ctx,
		sessionKey,
		roundID,
		reason,
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
		if len(s.Runtime.GetRunningRoundIDs(sessionKey)) == 0 {
			s.Permission.CancelRequestsForSession(sessionKey, "")
		}
	default:
		return fmt.Errorf("%w: unknown runtime outcome %q", ErrExactDMRoundInterruptUnsupported, result.Outcome)
	}
	s.finishInterrupt(ctx, sessionKey, roundID)
	return nil
}

func (s *Service) interruptSession(ctx context.Context, sessionKey string, resultText string) error {
	displayResultText := messagepkg.NormalizeInterruptDisplayText(resultText)
	roundIDs, err := s.Runtime.InterruptSession(ctx, sessionKey, resultText)
	switch {
	case err != nil && len(roundIDs) == 0:
		return err
	case err != nil:
		s.LoggerFor(ctx).Warn("DM 中断运行态失败，按失效进程清理",
			"session_key", sessionKey,
			"round_ids", roundIDs,
			"err", err,
		)
		if closeErr := s.Runtime.CloseSession(context.Background(), sessionKey); closeErr != nil {
			s.LoggerFor(ctx).Warn("DM 清理失效运行态 client 失败",
				"session_key", sessionKey,
				"err", closeErr,
			)
		}
		s.Permission.CancelRequestsForSession(sessionKey, displayResultText)
	case len(roundIDs) > 0:
		s.LoggerFor(ctx).Warn("中断 DM 会话运行轮次",
			"session_key", sessionKey,
			"round_count", len(roundIDs),
			"reason", displayResultText,
		)
		s.Permission.CancelRequestsForSession(sessionKey, displayResultText)
	}
	s.finishInterrupt(ctx, sessionKey, "")
	return nil
}

// finishInterrupt 在中断后刷新 session meta 的运行态并广播会话状态。
func (s *Service) finishInterrupt(ctx context.Context, sessionKey string, roundID string) {
	if err := s.refreshSessionMetaRuntimeStateByKey(ctx, sessionKey); err != nil {
		s.LoggerFor(ctx).Warn("DM 中断后刷新 session meta 失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"err", err,
		)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
}
