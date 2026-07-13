package workspace

import (
	"slices"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func alignTranscriptRoundMarkers(
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) []transcriptRoundMarker {
	if len(roundMarkers) == 0 {
		return nil
	}
	userTurns := collectTranscriptUserTurns(chain)
	if len(userTurns) == 0 {
		return nil
	}

	aligned := make([]transcriptRoundMarker, len(userTurns))
	used := make([]bool, len(roundMarkers))
	for index, turn := range userTurns {
		markerIndex := findMatchingRoundMarker(roundMarkers, used, turn)
		if markerIndex < 0 {
			continue
		}
		aligned[index] = roundMarkers[markerIndex]
		used[markerIndex] = true
	}

	fallback := tailTranscriptRoundMarkers(roundMarkers, len(userTurns))
	fallbackIndex := 0
	for index := range aligned {
		if strings.TrimSpace(aligned[index].RoundID) != "" || strings.TrimSpace(aligned[index].Content) != "" {
			continue
		}
		for fallbackIndex < len(fallback) {
			candidate := fallback[fallbackIndex]
			fallbackIndex++
			if markerAlreadyAligned(aligned, candidate) {
				continue
			}
			aligned[index] = candidate
			break
		}
	}
	return aligned
}

type transcriptUserTurn struct {
	Content   string
	Timestamp int64
}

func collectTranscriptUserTurns(chain []transcriptEntry) []transcriptUserTurn {
	turns := make([]transcriptUserTurn, 0)
	var lastTimestamp int64
	for _, entry := range chain {
		entryTimestamp := transcriptEntryTimestamp(entry.Data, entry.Index, lastTimestamp)
		lastTimestamp = entryTimestamp
		decoded, err := sdkprotocol.DecodeMessage(entry.Data)
		if err != nil {
			continue
		}
		if decoded.Type == sdkprotocol.MessageTypeUser &&
			!isTranscriptToolResult(decoded) &&
			shouldMaterializeTranscriptUserTurn(entry.Data) {
			turns = append(turns, transcriptUserTurn{
				Content:   sanitizeTranscriptUserContent(transcriptUserContent(entry.Data)),
				Timestamp: entryTimestamp,
			})
		}
	}
	return turns
}

func findMatchingRoundMarker(
	roundMarkers []transcriptRoundMarker,
	used []bool,
	turn transcriptUserTurn,
) int {
	content := strings.TrimSpace(turn.Content)
	if content == "" {
		return -1
	}
	bestIndex := -1
	var bestDistance int64
	for index, marker := range roundMarkers {
		if index < len(used) && used[index] {
			continue
		}
		if strings.TrimSpace(marker.Content) != content {
			continue
		}
		distance, ok := transcriptRoundMarkerDistance(turn.Timestamp, marker.Timestamp)
		if !ok {
			continue
		}
		if bestIndex < 0 || distance < bestDistance || (distance == bestDistance && index > bestIndex) {
			bestIndex = index
			bestDistance = distance
		}
	}
	return bestIndex
}

func transcriptRoundMarkerDistance(turnTimestamp int64, markerTimestamp int64) (int64, bool) {
	if turnTimestamp <= 0 || markerTimestamp <= 0 {
		return 0, true
	}
	// 允许少量落盘顺序抖动，但不要把新追加的 marker 绑定到旧 transcript user。
	const markerFutureToleranceMS = 5 * 1000
	if markerTimestamp > turnTimestamp+markerFutureToleranceMS {
		return 0, false
	}
	if turnTimestamp >= markerTimestamp {
		return turnTimestamp - markerTimestamp, true
	}
	return markerTimestamp - turnTimestamp, true
}

func tailTranscriptRoundMarkers(roundMarkers []transcriptRoundMarker, count int) []transcriptRoundMarker {
	if count <= 0 || len(roundMarkers) == 0 {
		return nil
	}
	if count >= len(roundMarkers) {
		return slices.Clone(roundMarkers)
	}
	startIndex := len(roundMarkers) - count
	return slices.Clone(roundMarkers[startIndex:])
}

func markerAlreadyAligned(aligned []transcriptRoundMarker, candidate transcriptRoundMarker) bool {
	for _, marker := range aligned {
		if strings.TrimSpace(marker.RoundID) == "" && strings.TrimSpace(marker.Content) == "" {
			continue
		}
		if marker.RoundID == candidate.RoundID &&
			marker.Content == candidate.Content &&
			marker.Timestamp == candidate.Timestamp &&
			marker.DeliveryPolicy == candidate.DeliveryPolicy &&
			marker.HiddenFromUser == candidate.HiddenFromUser {
			return true
		}
	}
	return false
}

func shouldMaterializeTranscriptUserTurn(entry map[string]any) bool {
	return sanitizeTranscriptUserContent(transcriptUserContent(entry)) != ""
}

func isTranscriptGoalContextOnlyUserTurn(entry map[string]any) bool {
	content := strings.TrimSpace(transcriptUserContent(entry))
	if strings.HasPrefix(content, "<goal_context>") &&
		strings.HasSuffix(content, "</goal_context>") {
		return true
	}
	return (strings.HasPrefix(content, "<internal_context source=\"goal\">") &&
		strings.HasSuffix(content, "</internal_context>")) ||
		(strings.HasPrefix(content, "<codex_internal_context source=\"goal\">") &&
			strings.HasSuffix(content, "</codex_internal_context>"))
}

func consumeTranscriptRoundMarker(markers []transcriptRoundMarker, index *int) transcriptRoundMarker {
	if index == nil {
		return transcriptRoundMarker{}
	}
	for *index < len(markers) {
		marker := markers[*index]
		*index++
		if strings.TrimSpace(marker.RoundID) != "" || strings.TrimSpace(marker.Content) != "" {
			return marker
		}
	}
	return transcriptRoundMarker{}
}
