// INPUT: 同 Agent 的 source/target DM Session、已完成 round_id 与预检 transcript 边界。
// OUTPUT: 先解析可持久化的 fork 依赖，再按该边界物化目标 Session。
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

// PrepareConversationFork 在 SQL 创建目标会话前验证轮次并解析稳定 transcript 边界。
func (s *Service) PrepareConversationFork(
	ctx context.Context,
	sourceSessionKey string,
	targetRoundID string,
) (string, string, error) {
	source, err := validateConversationForkSourceKey(sourceSessionKey)
	if err != nil {
		return "", "", err
	}
	targetRoundID = strings.TrimSpace(targetRoundID)
	if targetRoundID == "" {
		return "", "", errors.New("target round id is required")
	}

	agentValue, err := s.agents.GetAgent(ctx, source.AgentID)
	if err != nil {
		return "", "", err
	}
	sourceSession, err := s.ensureSession(ctx, agentValue, source, sourceSessionKey)
	if err != nil {
		return "", "", err
	}
	ownerHistory := s.history.ForOwner(agentValue.OwnerUserID)
	activeRoundIDs := s.runtime.GetRunningRoundIDs(sourceSessionKey)
	page, err := ownerHistory.ReadMessagesPageContext(
		ctx,
		agentValue.WorkspacePath,
		sourceSession,
		activeRoundIDs,
		workspacestore.HistoryPageQuery{
			AroundRoundID: targetRoundID,
			AroundLimit:   1,
		},
	)
	if err != nil {
		return "", "", fmt.Errorf("读取 source conversation 目标轮次: %w", err)
	}
	if !completedAssistantRound(page.Items, targetRoundID, activeRoundIDs) {
		return "", "", errors.New("目标轮次不支持分支，只能选择已完成的助手回复")
	}
	sourceSessionID, forkMessageID, err := resolveConversationForkBoundary(
		ownerHistory,
		agentValue.WorkspacePath,
		sourceSessionKey,
		sourceSession,
		targetRoundID,
	)
	if err != nil {
		return "", "", fmt.Errorf("解析 fork transcript 边界: %w", err)
	}
	return sourceSessionID, forkMessageID, nil
}

// ForkConversationSession 把已与 SQL 目标会话原子持久化的 fork 依赖物化到 workspace。
func (s *Service) ForkConversationSession(
	ctx context.Context,
	sourceSessionKey string,
	targetSessionKey string,
	targetRoundID string,
	sourceSessionID string,
	forkMessageID string,
) error {
	source, target, err := validateConversationForkKeys(sourceSessionKey, targetSessionKey)
	if err != nil {
		return err
	}
	targetRoundID = strings.TrimSpace(targetRoundID)
	sourceSessionID = strings.TrimSpace(sourceSessionID)
	forkMessageID = strings.TrimSpace(forkMessageID)
	if targetRoundID == "" || sourceSessionID == "" || forkMessageID == "" {
		return errors.New("conversation fork target and transcript boundary are required")
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
	if err = ownerHistory.ForkRoundMarkers(
		agentValue.WorkspacePath,
		sourceSessionKey,
		targetSessionKey,
		targetRoundID,
	); err != nil {
		return fmt.Errorf("复制 fork round marker: %w", err)
	}
	actualSessionID, actualMessageID, err := resolveConversationForkBoundary(
		ownerHistory,
		agentValue.WorkspacePath,
		targetSessionKey,
		sourceSession,
		targetRoundID,
	)
	if err != nil {
		return fmt.Errorf("重新验证 fork transcript 边界: %w", err)
	}
	if actualSessionID != sourceSessionID || actualMessageID != forkMessageID {
		return errors.New("fork transcript boundary changed before target materialization")
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
	source, err := validateConversationForkSourceKey(sourceRaw)
	if err != nil {
		return protocol.SessionKey{}, protocol.SessionKey{}, err
	}
	target := protocol.ParseSessionKey(targetRaw)
	if !validConversationForkKey(target) || source.AgentID != target.AgentID || source.Ref == target.Ref {
		return protocol.SessionKey{}, protocol.SessionKey{}, errors.New("conversation fork requires distinct DM sessions for the same Agent")
	}
	return source, target, nil
}

func validateConversationForkSourceKey(raw string) (protocol.SessionKey, error) {
	key := protocol.ParseSessionKey(raw)
	if !validConversationForkKey(key) {
		return protocol.SessionKey{}, errors.New("conversation fork requires a DM session")
	}
	return key, nil
}

func validConversationForkKey(key protocol.SessionKey) bool {
	return key.IsStructured &&
		key.Kind == protocol.SessionKeyKindAgent &&
		key.Channel == protocol.SessionChannelWebSocketSegment &&
		key.ChatType == protocol.RoomTypeDM &&
		strings.TrimSpace(key.AgentID) != "" &&
		strings.TrimSpace(key.Ref) != ""
}

func completedAssistantRound(rows []protocol.Message, roundID string, activeRoundIDs []string) bool {
	roundID = strings.TrimSpace(roundID)
	for _, activeRoundID := range activeRoundIDs {
		if strings.TrimSpace(activeRoundID) == roundID {
			return false
		}
	}

	hasAssistant := false
	hasNonSuccessfulTerminal := false
	for _, row := range rows {
		if protocol.MessageRoundID(row) != roundID {
			continue
		}
		switch protocol.MessageRole(row) {
		case "assistant":
			hasAssistant = true
			if summary, ok := row["result_summary"].(map[string]any); ok && len(summary) > 0 {
				hasNonSuccessfulTerminal = hasNonSuccessfulTerminal || !successfulForkResult(summary["subtype"])
			}
			if stopReason := strings.TrimSpace(forkStringValue(row["stop_reason"])); stopReason != "" {
				hasNonSuccessfulTerminal = hasNonSuccessfulTerminal ||
					stopReason == "cancelled" || stopReason == "interrupted" || stopReason == "error"
			}
		case "result":
			hasNonSuccessfulTerminal = hasNonSuccessfulTerminal || !successfulForkResult(row["subtype"])
		}
	}
	return hasAssistant && !hasNonSuccessfulTerminal
}

func successfulForkResult(value any) bool {
	status := strings.ToLower(strings.TrimSpace(forkStringValue(value)))
	return status != "" && status != "running" && status != "interrupted" && status != "cancelled" && status != "error"
}

func forkStringValue(value any) string {
	text, _ := value.(string)
	return text
}
