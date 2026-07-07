package workspace

import (
	"errors"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ReadMessages 读取 DM 历史。
func (s *AgentHistoryStore) ReadMessages(
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
) ([]protocol.Message, error) {
	rows, err := s.readHistoryRows(workspacePath, sessionValue)
	if err != nil {
		return nil, err
	}
	return normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs)), nil
}

// ReadMessagesPage 按 round 分页读取 DM 历史。
func (s *AgentHistoryStore) ReadMessagesPage(
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
	limit int,
	beforeRoundID string,
	beforeRoundTimestamp int64,
	aroundRoundID string,
	aroundLimit int,
) (protocol.MessagePage, error) {
	rows, err := s.readHistoryRows(workspacePath, sessionValue)
	if err != nil {
		return protocol.MessagePage{}, err
	}
	normalizedRows := normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs))
	if strings.TrimSpace(aroundRoundID) != "" {
		return paginateNormalizedHistoryRowsAround(
			normalizedRows,
			aroundRoundID,
			aroundLimit,
			false,
		), nil
	}
	return paginateNormalizedHistoryRows(
		normalizedRows,
		limit,
		beforeRoundID,
		beforeRoundTimestamp,
		false,
	), nil
}

func (s *AgentHistoryStore) readHistoryRows(
	workspacePath string,
	sessionValue protocol.Session,
) ([]protocol.Message, error) {
	sessionID := strings.TrimSpace(stringPointerValue(sessionValue.SessionID))
	overlayState, err := s.readOverlayHistoryState(workspacePath, sessionValue.SessionKey)
	if err != nil {
		return nil, err
	}
	if sessionID == "" {
		rows := buildOverlayOnlyHistoryRows(
			sessionValue.SessionKey,
			sessionValue.AgentID,
			overlayState.MessageRows,
			overlayState.RoundMarkers,
		)
		return applyHistoryRewrites(rows, overlayState.Rewrites), nil
	}

	transcriptRows, err := s.readTranscriptMessages(
		workspacePath,
		sessionValue.SessionKey,
		sessionValue.AgentID,
		sessionID,
		overlayState.RoundMarkers,
	)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// transcript 文件尚未出现时，只返回当前 overlay/round marker。
			rows := buildOverlayOnlyHistoryRows(
				sessionValue.SessionKey,
				sessionValue.AgentID,
				overlayState.MessageRows,
				overlayState.RoundMarkers,
			)
			return applyHistoryRewrites(rows, overlayState.Rewrites), nil
		}
		return nil, err
	}

	rows := mergeTranscriptAndOverlayRows(transcriptRows, overlayState.MessageRows)
	return applyHistoryRewrites(rows, overlayState.Rewrites), nil
}

func buildOverlayOnlyHistoryRows(
	sessionKey string,
	agentID string,
	overlayRows []protocol.Message,
	roundMarkers []transcriptRoundMarker,
) []protocol.Message {
	markerRows := materializeRoundMarkerMessages(sessionKey, agentID, roundMarkers)
	combined := make([]protocol.Message, 0, len(markerRows)+len(overlayRows))
	combined = append(combined, markerRows...)
	combined = append(combined, overlayRows...)
	return combined
}

func mergeTranscriptAndOverlayRows(
	transcriptRows []protocol.Message,
	overlayRows []protocol.Message,
) []protocol.Message {
	combined := make([]protocol.Message, 0, len(transcriptRows)+len(overlayRows))
	combined = append(combined, transcriptRows...)
	combined = append(combined, overlayRows...)
	return combined
}

func materializeRoundMarkerMessages(
	sessionKey string,
	agentID string,
	roundMarkers []transcriptRoundMarker,
) []protocol.Message {
	if len(roundMarkers) == 0 {
		return []protocol.Message{}
	}

	rows := make([]protocol.Message, 0, len(roundMarkers))
	for _, marker := range roundMarkers {
		roundID := strings.TrimSpace(marker.RoundID)
		if roundID == "" || marker.HiddenFromUser {
			continue
		}
		// 旧 marker 没有独立 user_message_id（历史上 message_id == round_id），
		// 读取时归一化为稳定派生 id，运行时不再出现两者相等的形状。
		userMessageID := strings.TrimSpace(marker.UserMessageID)
		if userMessageID == "" {
			userMessageID = "msg_user_" + roundID
		}
		row := protocol.Message{
			"message_id":  userMessageID,
			"session_key": sessionKey,
			"agent_id":    strings.TrimSpace(agentID),
			"round_id":    roundID,
			"role":        "user",
			"content":     strings.TrimSpace(marker.Content),
			"timestamp":   marker.Timestamp,
		}
		if agentRoundID := strings.TrimSpace(marker.AgentRoundID); agentRoundID != "" {
			row["agent_round_id"] = agentRoundID
		}
		if strings.TrimSpace(marker.DeliveryPolicy) != "" {
			row["delivery_policy"] = string(protocol.NormalizeChatDeliveryPolicy(marker.DeliveryPolicy))
		}
		if normalizedAttachments := protocol.NormalizeChatAttachments(marker.Attachments, agentID); len(normalizedAttachments) > 0 {
			row["attachments"] = normalizedAttachments
		}
		if len(marker.Metadata) > 0 {
			row["metadata"] = marker.Metadata
		}
		rows = append(rows, row)
	}
	return rows
}

func (s *AgentHistoryStore) readTranscriptMessages(
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
	roundMarkers []transcriptRoundMarker,
) ([]protocol.Message, error) {
	transcriptPath, err := s.resolveTranscriptPath(workspacePath, sessionID)
	if err != nil {
		return nil, err
	}
	fileInfo, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, err
	}

	roundMarkerFingerprint := fingerprintTranscriptRoundMarkers(roundMarkers)
	if cachedRows, ok := s.readTranscriptCache(transcriptPath, fileInfo, roundMarkerFingerprint); ok {
		return cachedRows, nil
	}

	entries, err := s.readTranscriptEntries(transcriptPath)
	if err != nil {
		return nil, err
	}
	chain := buildPrimaryTranscriptChain(entries)
	projectedRows := projectTranscriptChain(workspacePath, sessionKey, agentID, chain, roundMarkers)
	s.writeTranscriptCache(transcriptPath, fileInfo, roundMarkerFingerprint, projectedRows)
	return projectedRows, nil
}

// ReadTranscriptPathMessages 读取指定 transcript 文件并投影为 Nexus 消息。
func (s *AgentHistoryStore) ReadTranscriptPathMessages(
	transcriptPath string,
	workspacePath string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	transcriptPath = strings.TrimSpace(transcriptPath)
	if transcriptPath == "" {
		return []protocol.Message{}, nil
	}
	fileInfo, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, err
	}
	if cachedRows, ok := s.readTranscriptCache(transcriptPath, fileInfo, ""); ok {
		return cachedRows, nil
	}
	entries, err := s.readTranscriptEntries(transcriptPath)
	if err != nil {
		return nil, err
	}
	chain := buildPrimaryTranscriptChain(entries)
	projectedRows := projectTranscriptChain(workspacePath, sessionKey, agentID, chain, nil)
	s.writeTranscriptCache(transcriptPath, fileInfo, "", projectedRows)
	return projectedRows, nil
}
