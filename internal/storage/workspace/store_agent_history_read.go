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
) (protocol.MessagePage, error) {
	rows, err := s.readHistoryRows(workspacePath, sessionValue)
	if err != nil {
		return protocol.MessagePage{}, err
	}
	normalizedRows := normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs))
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
	overlayRows, roundMarkers, err := s.readOverlayRowsAndMarkers(workspacePath, sessionValue.SessionKey)
	if err != nil {
		return nil, err
	}
	if sessionID == "" {
		return buildOverlayOnlyHistoryRows(
			sessionValue.SessionKey,
			sessionValue.AgentID,
			overlayRows,
			roundMarkers,
		), nil
	}

	transcriptRows, err := s.readTranscriptMessages(
		workspacePath,
		sessionValue.SessionKey,
		sessionValue.AgentID,
		sessionID,
		roundMarkers,
	)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// transcript 文件尚未出现时，只返回当前 overlay/round marker。
			return buildOverlayOnlyHistoryRows(
				sessionValue.SessionKey,
				sessionValue.AgentID,
				overlayRows,
				roundMarkers,
			), nil
		}
		return nil, err
	}

	return mergeTranscriptAndOverlayRows(transcriptRows, overlayRows), nil
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
		row := protocol.Message{
			"message_id":  roundID,
			"session_key": sessionKey,
			"agent_id":    strings.TrimSpace(agentID),
			"round_id":    roundID,
			"role":        "user",
			"content":     strings.TrimSpace(marker.Content),
			"timestamp":   marker.Timestamp,
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
