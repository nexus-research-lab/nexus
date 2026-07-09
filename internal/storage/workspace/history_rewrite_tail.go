package workspace

import (
	"errors"
	"fmt"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// TranscriptRoundTail 描述一次 rewrite 需要从 SDK runtime 删除的 transcript 尾部。
type TranscriptRoundTail struct {
	TargetRoundID     string
	TargetMessageUUID string
	MessageUUIDs      []string
	RoundIDs          []string
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
	entries, err := s.readTranscriptEntries(transcriptPath)
	if err != nil {
		return TranscriptRoundTail{}, err
	}
	tail := resolveTranscriptRoundTail(
		buildPrimaryTranscriptChain(entries),
		overlayState.RoundMarkers,
		targetRoundID,
	)
	if tail.TargetMessageUUID == "" {
		return TranscriptRoundTail{}, fmt.Errorf("target round %s not found in transcript", targetRoundID)
	}
	if len(tail.MessageUUIDs) == 0 {
		return TranscriptRoundTail{}, fmt.Errorf("target round %s has no transcript uuid", targetRoundID)
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
	alignedMarkers := alignTranscriptRoundMarkers(chain, roundMarkers)
	markerIndex := 0
	found := false
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
