// INPUT: SDK transcript 主链、Nexus round marker 与目标 round_id。
// OUTPUT: 可安全删除的精确 transcript UUID 尾部，或可分类的边界缺失错误。
// POS: DM rewrite/fork 在修改 runtime 历史前的只读边界解析器。
package workspace

import (
	"errors"
	"fmt"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// ErrTranscriptRoundNotFound 表示目标 Nexus round 尚未物化进 SDK transcript。
// 调用方只能在另有 durable 失败证据时把它解释为 overlay-only rewrite；普通
// rewrite/fork 仍必须拒绝，不能把 transcript 损坏静默降级成无边界删除。
var ErrTranscriptRoundNotFound = errors.New("transcript round not found")

// TranscriptRoundTail 描述一次 rewrite 需要从 SDK runtime 删除的 transcript 尾部。
type TranscriptRoundTail struct {
	TargetRoundID      string
	TargetMessageUUID  string
	TargetRoundEndUUID string
	MessageUUIDs       []string
	RoundIDs           []string
}

// ResolveTranscriptRoundTail 将 Nexus round_id 解析成 SDK transcript UUID 尾部。
func (s *AgentHistoryStore) ResolveTranscriptRoundTail(
	workspacePath string,
	sessionKey string,
	sessionID string,
	targetRoundID string,
) (TranscriptRoundTail, error) {
	sessionID = strings.TrimSpace(sessionID)
	targetRoundID = strings.TrimSpace(targetRoundID)
	if sessionID == "" {
		return TranscriptRoundTail{}, errors.New("session id is required")
	}
	if targetRoundID == "" {
		return TranscriptRoundTail{}, errors.New("target round id is required")
	}

	overlayState, err := s.readOverlayHistoryState(workspacePath, sessionKey)
	if err != nil {
		return TranscriptRoundTail{}, err
	}
	transcriptPath, err := s.resolveTranscriptPath(workspacePath, sessionID)
	if err != nil {
		return TranscriptRoundTail{}, err
	}
	root, relative, _, err := s.openTranscriptPath(workspacePath, transcriptPath)
	if err != nil {
		return TranscriptRoundTail{}, err
	}
	defer root.Close()
	entries, err := s.readTranscriptEntriesAt(root, relative)
	if err != nil {
		return TranscriptRoundTail{}, err
	}
	tail := resolveTranscriptRoundTail(
		buildPrimaryTranscriptChain(entries),
		overlayState.RoundMarkers,
		targetRoundID,
	)
	if tail.TargetMessageUUID == "" {
		return TranscriptRoundTail{}, fmt.Errorf("%w: %s", ErrTranscriptRoundNotFound, targetRoundID)
	}
	if len(tail.MessageUUIDs) == 0 {
		return TranscriptRoundTail{}, fmt.Errorf("target round %s has no transcript uuid", targetRoundID)
	}
	if tail.TargetRoundEndUUID == "" {
		return TranscriptRoundTail{}, fmt.Errorf("target round %s has no transcript boundary", targetRoundID)
	}
	return tail, nil
}

func resolveTranscriptRoundTail(
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
	targetRoundID string,
) TranscriptRoundTail {
	targetRoundID = strings.TrimSpace(targetRoundID)
	if targetRoundID == "" {
		return TranscriptRoundTail{}
	}
	alignedMarkers := alignTranscriptRoundMarkers(
		chain,
		roundMarkers,
	)
	markerIndex := 0
	found := false
	targetEnded := false
	tail := TranscriptRoundTail{
		TargetRoundID: targetRoundID,
	}
	seenUUIDs := map[string]struct{}{}
	seenRoundIDs := map[string]struct{}{}

	for _, entry := range chain {
		if shouldSkipTranscriptEntry(entry.Data) {
			continue
		}
		entryRoundID := transcriptEntryRoundID(entry, alignedMarkers, &markerIndex)
		if !found {
			if entryRoundID != targetRoundID {
				continue
			}
			found = true
			tail.TargetMessageUUID = strings.TrimSpace(stringFromAny(entry.Data["uuid"]))
		} else if entryRoundID != "" && entryRoundID != targetRoundID {
			targetEnded = true
		}
		if !targetEnded {
			if uuid := strings.TrimSpace(stringFromAny(entry.Data["uuid"])); uuid != "" {
				tail.TargetRoundEndUUID = uuid
			}
		}
		appendTranscriptTailRoundID(&tail, seenRoundIDs, entryRoundID)
		appendTranscriptTailUUID(&tail, seenUUIDs, entry)
	}
	return tail
}

func transcriptEntryRoundID(
	entry transcriptEntry,
	alignedMarkers []transcriptRoundMarker,
	markerIndex *int,
) string {
	decoded, err := sdkprotocol.DecodeMessage(entry.Data)
	if err != nil || decoded.Type != sdkprotocol.MessageTypeUser {
		return ""
	}
	if isTranscriptToolResult(decoded) || !shouldMaterializeTranscriptUserTurn(entry.Data) {
		return ""
	}
	marker := consumeTranscriptRoundMarker(alignedMarkers, markerIndex)
	return firstNonEmpty(marker.RoundID, buildTranscriptRoundID(decoded.UUID))
}

func appendTranscriptTailUUID(tail *TranscriptRoundTail, seen map[string]struct{}, entry transcriptEntry) {
	uuid := strings.TrimSpace(stringFromAny(entry.Data["uuid"]))
	if uuid == "" {
		return
	}
	if _, exists := seen[uuid]; exists {
		return
	}
	seen[uuid] = struct{}{}
	tail.MessageUUIDs = append(tail.MessageUUIDs, uuid)
}

func appendTranscriptTailRoundID(tail *TranscriptRoundTail, seen map[string]struct{}, roundID string) {
	roundID = strings.TrimSpace(roundID)
	if roundID == "" {
		return
	}
	if _, exists := seen[roundID]; exists {
		return
	}
	seen[roundID] = struct{}{}
	tail.RoundIDs = append(tail.RoundIDs, roundID)
}
