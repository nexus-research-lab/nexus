// INPUT: transcript、overlay 与 round marker 状态。
// OUTPUT: 规范化并保留 source round 的 Agent 历史消息。
// POS: workspace Agent 历史读取与物化入口。
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ReadMessages 读取 DM 历史。
func (s *AgentHistoryStore) ReadMessages(
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
) ([]protocol.Message, error) {
	return withRuntimePermissionRepair(s, func() ([]protocol.Message, error) {
		rows, err := s.readHistoryRows(workspacePath, sessionValue)
		if err != nil {
			return nil, err
		}
		return normalizeHistoryRows(rows, normalizeActiveRoundIDs(activeRoundIDs)), nil
	})
}

// ReadMessagesPageContext 按 round 真分页读取 DM 历史，并允许请求取消索引校验与回建。
func (s *AgentHistoryStore) ReadMessagesPageContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	activeRoundIDs []string,
	query HistoryPageQuery,
) (protocol.MessagePage, error) {
	return withRuntimePermissionRepair(s, func() (protocol.MessagePage, error) {
		return readHistoryPageWithIndex(ctx, s.historyPageAccess(workspacePath, sessionValue), historyPageIndexRequest{
			Limit:                query.Limit,
			BeforeRoundID:        query.BeforeRoundID,
			BeforeRoundTimestamp: query.BeforeRoundTimestamp,
			AroundRoundID:        query.AroundRoundID,
			AroundLimit:          query.AroundLimit,
			ActiveRoundIDs:       activeRoundIDs,
			DeferIndex:           query.DeferIndex,
		})
	})
}

func (s *AgentHistoryStore) readHistoryRows(
	workspacePath string,
	sessionValue protocol.Session,
) ([]protocol.Message, error) {
	return s.readHistoryRowsContext(context.Background(), workspacePath, sessionValue)
}

func (s *AgentHistoryStore) readHistoryRowsContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
) ([]protocol.Message, error) {
	overlayState, err := s.readOverlayHistoryStateContext(ctx, workspacePath, sessionValue.SessionKey)
	if err != nil {
		return nil, err
	}
	sessionIDs := historyTranscriptSessionIDs(sessionValue)
	if len(sessionIDs) == 0 {
		rows := buildOverlayOnlyHistoryRows(
			sessionValue.SessionKey,
			sessionValue.AgentID,
			overlayState.MessageRows,
			overlayState.RoundMarkers,
		)
		return rows, nil
	}

	transcriptRows := make([]protocol.Message, 0)
	segmented, _ := sessionValue.Options[protocol.OptionRuntimeSegmentedTranscript].(bool)
	if segmented {
		transcriptRows, err = s.readSegmentedTranscriptMessagesContext(
			ctx,
			workspacePath,
			sessionValue.SessionKey,
			sessionValue.AgentID,
			sessionIDs,
			overlayState.RoundMarkers,
		)
		if err != nil {
			return nil, err
		}
	} else {
		sessionID := sessionIDs[0]
		_, forkMessageID := pendingForkTranscript(sessionValue)
		rows, readErr := s.readTranscriptMessagesContext(
			ctx,
			workspacePath,
			sessionValue.SessionKey,
			sessionValue.AgentID,
			sessionID,
			overlayState.RoundMarkers,
			forkMessageID,
		)
		if readErr != nil {
			if !errors.Is(readErr, os.ErrNotExist) {
				return nil, readErr
			}
		} else {
			transcriptRows = append(transcriptRows, rows...)
		}
	}

	rows := mergeTranscriptAndOverlayRows(
		transcriptRows,
		overlayState.MessageRows,
		materializeRoundMarkerMessages(
			sessionValue.SessionKey,
			sessionValue.AgentID,
			overlayState.RoundMarkers,
		),
	)
	return rows, nil
}

func (s *AgentHistoryStore) readSegmentedTranscriptMessages(
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionIDs []string,
	roundMarkers []transcriptRoundMarker,
) ([]protocol.Message, error) {
	return s.readSegmentedTranscriptMessagesContext(
		context.Background(),
		workspacePath,
		sessionKey,
		agentID,
		sessionIDs,
		roundMarkers,
	)
}

func (s *AgentHistoryStore) readSegmentedTranscriptMessagesContext(
	ctx context.Context,
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionIDs []string,
	roundMarkers []transcriptRoundMarker,
) ([]protocol.Message, error) {
	combinedChain := make([]transcriptEntry, 0)
	nextIndex := 0
	for _, sessionID := range sessionIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		transcriptPath, err := s.resolveTranscriptPathContext(ctx, workspacePath, sessionID)
		if err != nil {
			return nil, err
		}
		root, relative, _, err := s.openTranscriptPath(workspacePath, transcriptPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		entries, readErr := s.readTranscriptEntriesAtContext(ctx, root, relative)
		_ = root.Close()
		if readErr != nil {
			return nil, readErr
		}
		segmentChain := buildPrimaryTranscriptChain(entries)
		for index := range segmentChain {
			segmentChain[index].Index = nextIndex
			nextIndex++
		}
		combinedChain = append(combinedChain, segmentChain...)
	}
	return projectTranscriptChain(workspacePath, sessionKey, agentID, combinedChain, roundMarkers), nil
}

func historyTranscriptSessionIDs(sessionValue protocol.Session) []string {
	if sourceSessionID, messageID := pendingForkTranscript(sessionValue); sourceSessionID != "" && messageID != "" {
		return []string{sourceSessionID}
	}
	segmented, _ := sessionValue.Options[protocol.OptionRuntimeSegmentedTranscript].(bool)
	if segmented {
		return protocol.SessionTranscriptIDs(sessionValue)
	}
	current := strings.TrimSpace(stringPointerValue(sessionValue.SessionID))
	if current == "" {
		return nil
	}
	return []string{current}
}

func pendingForkTranscript(sessionValue protocol.Session) (string, string) {
	sourceSessionID, _ := sessionValue.Options[protocol.OptionRuntimeForkSourceSessionID].(string)
	messageID, _ := sessionValue.Options[protocol.OptionRuntimeForkMessageID].(string)
	return strings.TrimSpace(sourceSessionID), strings.TrimSpace(messageID)
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
	roundMarkerRows []protocol.Message,
) []protocol.Message {
	combined := make(
		[]protocol.Message,
		0,
		len(transcriptRows)+len(overlayRows)+len(roundMarkerRows),
	)
	// 可见 marker 是 durable 用户输入真相；即使 transcript 断链或无法解码，
	// 也必须进入统一 compact，由稳定 message_id 与 transcript 用户行去重。
	combined = append(combined, transcriptRows...)
	combined = append(combined, roundMarkerRows...)
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
		if clientMessageID := strings.TrimSpace(marker.ClientMessageID); clientMessageID != "" {
			row["client_message_id"] = clientMessageID
		}
		if sourceRoundID := strings.TrimSpace(marker.SourceRoundID); sourceRoundID != "" {
			row["source_round_id"] = sourceRoundID
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
		if marker.ControlOnly {
			row["control_only"] = true
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
	throughMessageID string,
) ([]protocol.Message, error) {
	return s.readTranscriptMessagesContext(
		context.Background(),
		workspacePath,
		sessionKey,
		agentID,
		sessionID,
		roundMarkers,
		throughMessageID,
	)
}

func (s *AgentHistoryStore) readTranscriptMessagesContext(
	ctx context.Context,
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
	roundMarkers []transcriptRoundMarker,
	throughMessageID string,
) ([]protocol.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	transcriptPath, err := s.resolveTranscriptPathContext(ctx, workspacePath, sessionID)
	if err != nil {
		return nil, err
	}
	root, relative, fileInfo, err := s.openTranscriptPath(workspacePath, transcriptPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	throughMessageID = strings.TrimSpace(throughMessageID)
	roundMarkerFingerprint := throughMessageID + "\n" + fingerprintTranscriptRoundMarkers(roundMarkers)
	if cachedRows, ok := s.readTranscriptCache(transcriptPath, fileInfo, roundMarkerFingerprint); ok {
		return cachedRows, nil
	}

	entries, err := s.readTranscriptEntriesAtContext(ctx, root, relative)
	if err != nil {
		return nil, err
	}
	chain := buildPrimaryTranscriptChain(entries)
	if throughMessageID != "" {
		var found bool
		chain, found = transcriptChainThroughMessage(chain, throughMessageID)
		if !found {
			return nil, fmt.Errorf("fork transcript boundary %s not found", throughMessageID)
		}
	}
	projectedRows := projectTranscriptChain(workspacePath, sessionKey, agentID, chain, roundMarkers)
	s.writeTranscriptCache(transcriptPath, fileInfo, roundMarkerFingerprint, projectedRows)
	return projectedRows, nil
}

func transcriptChainThroughMessage(chain []transcriptEntry, messageID string) ([]transcriptEntry, bool) {
	for index, entry := range chain {
		if strings.TrimSpace(stringFromAny(entry.Data["uuid"])) == messageID {
			return chain[:index+1], true
		}
	}
	return nil, false
}

// ReadTranscriptSessionMessages 按受控 session id 定位独立 Agent thread，
// 并使用普通 transcript 投影保留消息、思考、工具调用和工具结果。
func (s *AgentHistoryStore) ReadTranscriptSessionMessages(
	workspacePath string,
	transcriptSessionID string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	return withRuntimePermissionRepair(s, func() ([]protocol.Message, error) {
		transcriptSessionID = strings.ToLower(strings.TrimSpace(transcriptSessionID))
		if !IsTranscriptSessionID(transcriptSessionID) &&
			!IsSubagentTranscriptSessionID(transcriptSessionID) {
			return []protocol.Message{}, nil
		}
		transcriptPath, err := s.resolveTranscriptPath(workspacePath, transcriptSessionID)
		if err != nil {
			return nil, err
		}
		return s.readTranscriptPathMessages(
			transcriptPath,
			workspacePath,
			sessionKey,
			agentID,
		)
	})
}

// ReadTranscriptPathMessages 读取指定 transcript 文件并投影为 Nexus 消息。
func (s *AgentHistoryStore) ReadTranscriptPathMessages(
	transcriptPath string,
	workspacePath string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	return withRuntimePermissionRepair(s, func() ([]protocol.Message, error) {
		return s.readTranscriptPathMessages(
			transcriptPath,
			workspacePath,
			sessionKey,
			agentID,
		)
	})
}

// ReadTranscriptPathMessagesForOwner 从 owner 固定的 workspace/runtime 根读取
// 显式 transcript，避免在请求路径上重新解析可被替换的用户目录。
func (s *AgentHistoryStore) ReadTranscriptPathMessagesForOwner(
	ownerUserID string,
	transcriptPath string,
	workspacePath string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	ownerHistory := s
	if strings.TrimSpace(ownerUserID) != strings.TrimSpace(s.ownerUserID) {
		ownerHistory = s.ForOwner(ownerUserID)
	}
	return withRuntimePermissionRepair(ownerHistory, func() ([]protocol.Message, error) {
		candidates, closeRoots, err := s.openOwnerTranscriptCandidates(
			ownerUserID,
			workspacePath,
		)
		if err != nil {
			return nil, err
		}
		defer closeRoots()
		return s.readTranscriptPathMessagesAt(
			candidates,
			transcriptPath,
			workspacePath,
			sessionKey,
			agentID,
		)
	})
}

func (s *AgentHistoryStore) readTranscriptPathMessages(
	transcriptPath string,
	workspacePath string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	return s.readTranscriptPathMessagesAt(
		nil,
		transcriptPath,
		workspacePath,
		sessionKey,
		agentID,
	)
}

func (s *AgentHistoryStore) readTranscriptPathMessagesAt(
	candidates []transcriptRootCandidate,
	transcriptPath string,
	workspacePath string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	const explicitTranscriptCacheKey = "explicit-transcript"

	transcriptPath = strings.TrimSpace(transcriptPath)
	if transcriptPath == "" {
		return []protocol.Message{}, nil
	}
	var (
		root     *confinedfs.Root
		relative string
		fileInfo os.FileInfo
		err      error
	)
	if candidates == nil {
		root, relative, fileInfo, err = s.openTranscriptPath(workspacePath, transcriptPath)
	} else {
		root, relative, fileInfo, err = openTranscriptPathAt(candidates, transcriptPath)
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if cachedRows, ok := s.readTranscriptCache(transcriptPath, fileInfo, explicitTranscriptCacheKey); ok {
		return cachedRows, nil
	}
	entries, err := s.readTranscriptEntriesAt(root, relative)
	if err != nil {
		return nil, err
	}
	chain := buildExplicitTranscriptChain(entries)
	projectedRows := projectExplicitTranscriptChain(workspacePath, sessionKey, agentID, chain)
	projectedRows = normalizeHistoryRows(projectedRows, nil)
	s.writeTranscriptCache(transcriptPath, fileInfo, explicitTranscriptCacheKey, projectedRows)
	return projectedRows, nil
}

// ReadTranscriptLinkMessages 投影 runtime 明确返回的 transcript 符号链接。
// 链接入口与最终目标分别绑定到授权根，普通 transcript 读取仍拒绝符号链接。
func (s *AgentHistoryStore) ReadTranscriptLinkMessages(
	transcriptPath string,
	workspacePath string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	return withRuntimePermissionRepair(s, func() ([]protocol.Message, error) {
		targetPath, err := s.resolveTranscriptLinkTarget(workspacePath, transcriptPath)
		if err != nil {
			return nil, err
		}
		return s.readTranscriptPathMessages(
			targetPath,
			workspacePath,
			sessionKey,
			agentID,
		)
	})
}

// ReadTranscriptLinkMessagesForOwner 在同一组 owner 固定目录句柄内解析链接并
// 读取最终 transcript，避免链接校验与目标打开之间重新经过绝对路径。
func (s *AgentHistoryStore) ReadTranscriptLinkMessagesForOwner(
	ownerUserID string,
	transcriptPath string,
	workspacePath string,
	sessionKey string,
	agentID string,
) ([]protocol.Message, error) {
	ownerHistory := s
	if strings.TrimSpace(ownerUserID) != strings.TrimSpace(s.ownerUserID) {
		ownerHistory = s.ForOwner(ownerUserID)
	}
	return withRuntimePermissionRepair(ownerHistory, func() ([]protocol.Message, error) {
		candidates, closeRoots, err := s.openOwnerTranscriptCandidates(
			ownerUserID,
			workspacePath,
		)
		if err != nil {
			return nil, err
		}
		defer closeRoots()
		targetPath, err := resolveTranscriptLinkTargetAt(candidates, transcriptPath)
		if err != nil {
			return nil, err
		}
		return s.readTranscriptPathMessagesAt(
			candidates,
			targetPath,
			workspacePath,
			sessionKey,
			agentID,
		)
	})
}
