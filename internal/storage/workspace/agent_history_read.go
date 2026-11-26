// INPUT: transcript、overlay 与 round marker 状态。
// OUTPUT: 规范化并保留 source round 的 Agent 历史消息；非 fork 读取经投影前缀检查点只重放新增轮次，并可附带读模型检查点候选。
// POS: workspace Agent 历史读取与物化入口。
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/message"
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
	parts, err := s.readHistoryRowPartsContext(ctx, workspacePath, sessionValue, false)
	if err != nil {
		return nil, err
	}
	return parts.merged(), nil
}

// agentHistoryRowParts 是合并前按来源分开的原始历史行，合并顺序固定为
// transcript、marker、overlay。
type agentHistoryRowParts struct {
	Transcript []protocol.Message
	// Markers 是全部 round marker 物化出的行及其 marker 下标。
	Markers historyTailPart
	Overlay []protocol.Message
	// Replay 是 transcript 投影的检查点候选；只有调用方要求且读取的是单个非 fork
	// transcript 时才非空，读模型据此持久化投影位置。
	Replay *transcriptReplay
}

func (p agentHistoryRowParts) merged() []protocol.Message {
	return mergeTranscriptAndOverlayRows(p.Transcript, p.Overlay, p.Markers.rows)
}

func (s *AgentHistoryStore) readHistoryRowPartsContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	withReplay bool,
) (agentHistoryRowParts, error) {
	parts, overlayState, err := s.readOverlayRowPartsContext(ctx, workspacePath, sessionValue)
	if err != nil {
		return agentHistoryRowParts{}, err
	}
	sessionIDs := historyTranscriptSessionIDs(sessionValue)
	if len(sessionIDs) == 0 {
		return parts, nil
	}
	segmented, _ := sessionValue.Options[protocol.OptionRuntimeSegmentedTranscript].(bool)
	if segmented {
		parts.Transcript, err = s.readSegmentedTranscriptMessagesContext(
			ctx,
			workspacePath,
			sessionValue.SessionKey,
			sessionValue.AgentID,
			sessionIDs,
			overlayState.RoundMarkers,
		)
		return parts, err
	}
	_, forkMessageID := pendingForkTranscript(sessionValue)
	rows, replay, readErr := s.readTranscriptReplayContext(
		ctx,
		workspacePath,
		sessionValue.SessionKey,
		sessionValue.AgentID,
		sessionIDs[0],
		overlayState.RoundMarkers,
		forkMessageID,
		withReplay,
	)
	if readErr != nil {
		if !errors.Is(readErr, os.ErrNotExist) {
			return agentHistoryRowParts{}, readErr
		}
		return parts, nil
	}
	parts.Transcript = rows
	parts.Replay = replay
	return parts, nil
}

// readOverlayRowPartsContext 读取 overlay 派生的 marker 行与消息行，Transcript 留空。
func (s *AgentHistoryStore) readOverlayRowPartsContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
) (agentHistoryRowParts, overlayHistoryState, error) {
	overlayState, err := s.readOverlayHistoryStateContext(ctx, workspacePath, sessionValue.SessionKey)
	if err != nil {
		return agentHistoryRowParts{}, overlayHistoryState{}, err
	}
	markers, _ := appendedHistoryTailPositions(len(overlayState.RoundMarkers), nil, 0)
	return agentHistoryRowParts{
		Transcript: []protocol.Message{},
		Markers: roundMarkerHistoryTailPart(
			sessionValue.SessionKey,
			sessionValue.AgentID,
			overlayState.RoundMarkers,
			markers,
		),
		Overlay: overlayState.MessageRows,
	}, overlayState, nil
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
	current := textutil.PointerValue(sessionValue.SessionID)
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

// roundMarkerHistoryTailPart 物化 positions 指定的 round marker（下标指 roundMarkers）。
func roundMarkerHistoryTailPart(
	sessionKey string,
	agentID string,
	roundMarkers []transcriptRoundMarker,
	positions []int,
) historyTailPart {
	part := historyTailPart{rows: make([]protocol.Message, 0, len(positions)), total: len(roundMarkers)}
	for _, position := range positions {
		if row, ok := materializeRoundMarkerMessage(sessionKey, agentID, roundMarkers[position]); ok {
			part.rows = append(part.rows, row)
			part.positions = append(part.positions, position)
		}
	}
	return part
}

// materializeRoundMarkerMessage 把一个 round marker 物化成用户消息行；隐藏或没有 round_id 的 marker 不产生行。
func materializeRoundMarkerMessage(sessionKey string, agentID string, marker transcriptRoundMarker) (protocol.Message, bool) {
	roundID := strings.TrimSpace(marker.RoundID)
	if roundID == "" || marker.HiddenFromUser {
		return nil, false
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
	if marker.InputSeq > 0 {
		row[protocol.InputSeqField] = marker.InputSeq
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
	if message.IsRuntimeWakeMetadata(marker.Metadata) {
		message.DecorateRuntimeWakeUserMessage(row, message.RuntimeWakeTriggerFromMetadata(marker.Content, marker.Metadata))
	}
	return row, true
}

// readTranscriptReplayContext 返回完整投影；withReplay 时额外返回读模型检查点候选
// （fork 截断读取、没有用户轮次或主链乱序时为 nil）。
func (s *AgentHistoryStore) readTranscriptReplayContext(
	ctx context.Context,
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
	roundMarkers []transcriptRoundMarker,
	throughMessageID string,
	withReplay bool,
) ([]protocol.Message, *transcriptReplay, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	transcriptPath, err := s.resolveTranscriptPathContext(ctx, workspacePath, sessionID)
	if err != nil {
		return nil, nil, err
	}
	root, relative, _, err := s.openTranscriptPath(workspacePath, transcriptPath)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()

	throughMessageID = strings.TrimSpace(throughMessageID)
	entries, cachePath, lineage, err := s.readTranscriptEntriesLineageAtContext(ctx, root, relative)
	if err != nil {
		return nil, nil, err
	}
	chain := buildPrimaryTranscriptChain(entries)
	if throughMessageID != "" {
		var found bool
		chain, found = transcriptChainThroughMessage(chain, throughMessageID)
		if !found {
			return nil, nil, fmt.Errorf("fork transcript boundary %s not found", throughMessageID)
		}
		return projectTranscriptChain(workspacePath, sessionKey, agentID, cloneTranscriptEntries(chain), roundMarkers), nil, nil
	}
	// 长会话每轮只追加一个用户轮次，检查点让重建只投影最后一轮之后的条目。
	projection := s.projectTranscriptChainCheckpointed(cachePath, lineage, workspacePath, sessionKey, agentID, chain, roundMarkers)
	if !withReplay {
		return projection.Rows, nil, nil
	}
	file, err := s.openTranscriptReadFile(root, relative)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	replay, err := newTranscriptReplay(ctx, file, info.Size(), entries, chain, projection)
	return projection.Rows, replay, err
}

func transcriptChainThroughMessage(chain []transcriptEntry, messageID string) ([]transcriptEntry, bool) {
	for index, entry := range chain {
		if stringFromAny(entry.Data["uuid"]) == messageID {
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
	transcriptPath = strings.TrimSpace(transcriptPath)
	if transcriptPath == "" {
		return []protocol.Message{}, nil
	}
	var (
		root     *confinedfs.Root
		relative string
		err      error
	)
	if candidates == nil {
		root, relative, _, err = s.openTranscriptPath(workspacePath, transcriptPath)
	} else {
		root, relative, _, err = openTranscriptPathAt(candidates, transcriptPath)
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := s.readTranscriptEntriesAt(root, relative)
	if err != nil {
		return nil, err
	}
	chain := buildExplicitTranscriptChain(entries)
	projectedRows := projectExplicitTranscriptChain(workspacePath, sessionKey, agentID, chain)
	projectedRows = normalizeHistoryRows(projectedRows, nil)
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
