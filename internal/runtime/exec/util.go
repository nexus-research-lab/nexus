package exec

import (
	"context"
	"errors"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const (
	defaultAssistantTerminalGrace     = 1500 * time.Millisecond
	defaultInterruptedTerminalTimeout = 2 * time.Second
	defaultRoundIdleTimeout           = 5 * time.Minute
)

func roundQueryContent(request RoundExecutionRequest) any {
	if request.Content != nil {
		return request.Content
	}
	return request.Query
}

func roundResultWithElapsed(result RoundExecutionResult, startedAt time.Time) RoundExecutionResult {
	if result.ElapsedTimeSeconds > 0 || startedAt.IsZero() {
		return result
	}
	result.ElapsedTimeSeconds = int64(time.Since(startedAt).Seconds())
	return result
}

func normalizeAssistantTerminalGrace(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return defaultAssistantTerminalGrace
}

func normalizeInterruptedTerminalTimeout(value time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return defaultInterruptedTerminalTimeout
}

func normalizeRoundIdleTimeout(timeout time.Duration) time.Duration {
	if timeout < 0 {
		return 0
	}
	if timeout == 0 {
		return defaultRoundIdleTimeout
	}
	return timeout
}

func isRoundAbortError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, agentclient.ErrAborted)
}

func resetRoundIdleTimer(timer *time.Timer, timeout time.Duration) {
	if timer == nil || timeout <= 0 {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}

func abortRoundClientAfterIdleTimeout(client Client) {
	if client == nil {
		return
	}
	interruptCtx, interruptCancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	_ = client.Interrupt(interruptCtx)
	interruptCancel()

	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	_ = client.Disconnect(disconnectCtx)
	disconnectCancel()
}

func disconnectUncleanRoundClient(client Client) {
	if client == nil {
		return
	}
	if runtimectx.DiscardUncleanClientSession(client) {
		return
	}
	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), runtimectx.RoundIdleAbortTimeout)
	_ = client.Disconnect(disconnectCtx)
	disconnectCancel()
}

func shouldTreatAsInterrupted(ctx context.Context, interruptReason func() string) bool {
	return ctx.Err() != nil || strings.TrimSpace(resolveInterruptReason(interruptReason)) != ""
}

func resolveInterruptReason(interruptReason func() string) string {
	if interruptReason == nil {
		return ""
	}
	return strings.TrimSpace(interruptReason())
}

func resolveSessionID(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func messageString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}
