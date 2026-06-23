package room

import (
	"context"
	"log/slog"
	"strings"

	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
)

func (s *RealtimeService) resolveReusableRoomSDKSessionID(
	ctx context.Context,
	logger *slog.Logger,
	workspacePath string,
	slot *activeRoomSlot,
	resumeID string,
) (string, error) {
	resumeID = strings.TrimSpace(resumeID)
	if resumeID == "" {
		return "", nil
	}
	decision := sessionresumesvc.NewPolicy(s.history).CanResume(workspacePath, resumeID)
	if decision.Allowed {
		return resumeID, nil
	}
	if decision.Err != nil {
		logger.Warn("检查 Room SDK session transcript 失败，跳过过期 resume",
			"agent_id", slot.AgentID,
			"agent_round_id", slot.AgentRoundID,
			"runtime_session_key", slot.RuntimeSessionKey,
			"room_session_id", slot.RoomSessionID,
			"workspace_path", workspacePath,
			"sdk_session_id", decision.SessionID,
			"reason", string(decision.Reason),
			"err", decision.Err,
		)
		if clearErr := s.clearSlotSDKSessionID(ctx, slot); clearErr != nil {
			return "", clearErr
		}
		return "", nil
	}

	logger.Warn("Room SDK session transcript 不存在，跳过过期 resume",
		"agent_id", slot.AgentID,
		"agent_round_id", slot.AgentRoundID,
		"runtime_session_key", slot.RuntimeSessionKey,
		"room_session_id", slot.RoomSessionID,
		"workspace_path", workspacePath,
		"sdk_session_id", decision.SessionID,
		"reason", string(decision.Reason),
	)
	if err := s.clearSlotSDKSessionID(ctx, slot); err != nil {
		return "", err
	}
	return "", nil
}
