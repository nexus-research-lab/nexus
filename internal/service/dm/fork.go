// INPUT: 同 Agent 的 source/target DM Session 与一个已完成 round_id。
// OUTPUT: 按该轮 transcript 边界持久化待首次输入物化的分支 Session。
// POS: Room conversation fork 到 runtime transcript fork 的适配层。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// ForkConversationSession 从 source 的已完成轮次创建待首次输入物化的 target session。
func (s *Service) ForkConversationSession(
	ctx context.Context,
	sourceSessionKey string,
	targetSessionKey string,
	targetRoundID string,
) error {
	source, target, err := validateConversationForkKeys(sourceSessionKey, targetSessionKey)
	if err != nil {
		return err
	}
	targetRoundID = strings.TrimSpace(targetRoundID)
	if targetRoundID == "" {
		return errors.New("target round id is required")
	}

	agentValue, err := s.agents.GetAgent(ctx, source.AgentID)
	if err != nil {
		return err
	}
	sourceSession, err := s.ensureSession(ctx, agentValue, source, sourceSessionKey)
	if err != nil {
		return err
	}
	targetSession, err := s.ensureSession(ctx, agentValue, target, targetSessionKey)
	if err != nil {
		return err
	}
	if dmdomain.StringPointerValue(targetSession.SessionID) != "" {
		return errors.New("target conversation already has an SDK session")
	}
	if targetSession.Options == nil {
		targetSession.Options = map[string]any{}
	}
	runtimeFingerprintFromSession(sourceSession).apply(targetSession.Options)
	ownerHistory := s.history.ForOwner(agentValue.OwnerUserID)
	activeRoundIDs := s.runtime.GetRunningRoundIDs(sourceSessionKey)
	rows, err := ownerHistory.ReadMessages(
		agentValue.WorkspacePath,
		sourceSession,
		activeRoundIDs,
	)
	if err != nil {
		return fmt.Errorf("读取 source conversation 历史: %w", err)
	}
	if !completedAssistantRound(rows, targetRoundID, activeRoundIDs) {
		return errors.New("目标轮次不支持分支，只能选择已完成的助手回复")
	}
	sourceSessionID, forkMessageID, err := resolveConversationForkBoundary(
		ownerHistory,
		agentValue.WorkspacePath,
		sourceSessionKey,
		sourceSession,
		targetRoundID,
	)
	if err != nil {
		return fmt.Errorf("解析 fork transcript 边界: %w", err)
	}
	if err = ownerHistory.ForkRoundMarkers(
		agentValue.WorkspacePath,
		sourceSessionKey,
		targetSessionKey,
		targetRoundID,
	); err != nil {
		return fmt.Errorf("复制 fork round marker: %w", err)
	}
	targetSession.Options[protocol.OptionRuntimeForkSourceSessionID] = sourceSessionID
	targetSession.Options[protocol.OptionRuntimeForkMessageID] = forkMessageID
	forkRows, err := ownerHistory.ReadMessages(agentValue.WorkspacePath, targetSession, nil)
	if err != nil {
		return fmt.Errorf("读取 fork conversation 历史: %w", err)
	}
	targetSession.MessageCount = len(forkRows)
	_, err = s.files.ForOwner(agentValue.OwnerUserID).PatchSessionRuntime(
		agentValue.WorkspacePath,
		targetSession,
	)
	if err != nil {
		return fmt.Errorf("持久化 fork conversation: %w", err)
	}
	return nil
}

func pendingConversationFork(options map[string]any) (string, string) {
	sourceSessionID, _ := options[protocol.OptionRuntimeForkSourceSessionID].(string)
	messageID, _ := options[protocol.OptionRuntimeForkMessageID].(string)
	return strings.TrimSpace(sourceSessionID), strings.TrimSpace(messageID)
}

func resolveConversationForkBoundary(
	history *workspacestore.AgentHistoryStore,
	workspacePath string,
	sessionKey string,
	sourceSession protocol.Session,
	targetRoundID string,
) (string, string, error) {
	sessionIDs := []string{dmdomain.StringPointerValue(sourceSession.SessionID)}
	if segmented, _ := sourceSession.Options[protocol.OptionRuntimeSegmentedTranscript].(bool); segmented {
		sessionIDs = protocol.SessionTranscriptIDs(sourceSession)
	}
	var resolveErr error
	for _, sessionID := range sessionIDs {
		if strings.TrimSpace(sessionID) == "" {
			continue
		}
		tail, err := history.ResolveTranscriptRoundTail(
			workspacePath,
			sessionKey,
			sessionID,
			targetRoundID,
		)
		if err == nil {
			return sessionID, tail.TargetRoundEndUUID, nil
		}
		resolveErr = errors.Join(resolveErr, err)
	}
	if resolveErr == nil {
		return "", "", errors.New("source conversation has no SDK session")
	}
	return "", "", resolveErr
}

func validateConversationForkKeys(sourceRaw string, targetRaw string) (protocol.SessionKey, protocol.SessionKey, error) {
	source := protocol.ParseSessionKey(sourceRaw)
	target := protocol.ParseSessionKey(targetRaw)
	valid := func(key protocol.SessionKey) bool {
		return key.IsStructured &&
			key.Kind == protocol.SessionKeyKindAgent &&
			key.Channel == protocol.SessionChannelWebSocketSegment &&
			key.ChatType == protocol.RoomTypeDM &&
			strings.TrimSpace(key.AgentID) != "" &&
			strings.TrimSpace(key.Ref) != ""
	}
	if !valid(source) || !valid(target) || source.AgentID != target.AgentID || source.Ref == target.Ref {
		return protocol.SessionKey{}, protocol.SessionKey{}, errors.New("conversation fork requires distinct DM sessions for the same Agent")
	}
	return source, target, nil
}

func completedAssistantRound(rows []protocol.Message, roundID string, activeRoundIDs []string) bool {
	for _, turn := range workspacestore.ProjectConversationTurns(rows, false, activeRoundIDs) {
		if turn.RoundID != roundID || turn.Status != "finished" {
			continue
		}
		for _, slot := range turn.AgentSlots {
			if len(slot.AssistantMessages) > 0 {
				return true
			}
		}
		return false
	}
	return false
}
