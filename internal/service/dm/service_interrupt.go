package dm

import (
	"context"

	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// HandleInterrupt 处理中断请求。
func (s *Service) HandleInterrupt(ctx context.Context, request InterruptRequest) error {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return err
	}
	return s.interruptSession(ctx, sessionKey, messagepkg.InterruptWithoutMessage)
}

func (s *Service) interruptSession(ctx context.Context, sessionKey string, resultText string) error {
	roundIDs, err := s.runtime.InterruptSession(ctx, sessionKey, resultText)
	displayResultText := resultText
	if displayResultText == messagepkg.InterruptWithoutMessage {
		displayResultText = ""
	}
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
