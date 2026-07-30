// INPUT: 已筛选 transcript 链、durable round marker 与用户输入时间。
// OUTPUT: 与正式历史投影一一对应且不会跨可见性错绑的 round marker。
// POS: transcript 用户轮次与 Nexus durable round 身份的唯一对齐边界。
package workspace

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func alignTranscriptRoundMarkers(
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
) []transcriptRoundMarker {
	return alignTranscriptRoundMarkersWithFilter(chain, roundMarkers, shouldSkipTranscriptEntry)
}

func alignTranscriptRoundMarkersWithFilter(
	chain []transcriptEntry,
	roundMarkers []transcriptRoundMarker,
	shouldSkip func(map[string]any) bool,
) []transcriptRoundMarker {
	if len(roundMarkers) == 0 {
		return nil
	}
	userTurns := collectTranscriptUserTurns(chain, shouldSkip)
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

	for index, turn := range userTurns {
		if transcriptRoundMarkerPresent(aligned[index]) {
			continue
		}
		markerIndex := findCompatibleFallbackRoundMarker(roundMarkers, used, turn)
		if markerIndex < 0 {
			continue
		}
		aligned[index] = roundMarkers[markerIndex]
		used[markerIndex] = true
	}
	return aligned
}

type transcriptUserTurn struct {
	Content         string
	Timestamp       int64
	GoalContextOnly bool
}

func collectTranscriptUserTurns(
	chain []transcriptEntry,
	shouldSkip func(map[string]any) bool,
) []transcriptUserTurn {
	turns := make([]transcriptUserTurn, 0)
	var lastTimestamp int64
	for _, entry := range chain {
		if shouldSkip != nil && shouldSkip(entry.Data) {
			continue
		}
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
				Content:         sanitizeTranscriptUserContent(transcriptUserContent(entry.Data)),
				Timestamp:       entryTimestamp,
				GoalContextOnly: isTranscriptGoalContextOnlyUserTurn(entry.Data),
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
		if sanitizeTranscriptUserContent(marker.Content) != content {
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

func findCompatibleFallbackRoundMarker(
	roundMarkers []transcriptRoundMarker,
	used []bool,
	turn transcriptUserTurn,
) int {
	const markerFallbackMaxDistanceMS = 10 * 60 * 1000

	bestIndex := -1
	var bestDistance int64
	for index, marker := range roundMarkers {
		if index < len(used) && used[index] {
			continue
		}
		if !roundMarkerFallbackCompatible(turn, marker) {
			continue
		}
		distance, ok := transcriptRoundMarkerDistance(turn.Timestamp, marker.Timestamp)
		if !ok {
			continue
		}
		if !turn.GoalContextOnly &&
			turn.Timestamp > 0 &&
			marker.Timestamp > 0 &&
			distance > markerFallbackMaxDistanceMS {
			continue
		}
		if bestIndex < 0 || distance < bestDistance || (distance == bestDistance && index > bestIndex) {
			bestIndex = index
			bestDistance = distance
		}
	}
	return bestIndex
}

func roundMarkerFallbackCompatible(turn transcriptUserTurn, marker transcriptRoundMarker) bool {
	if turn.GoalContextOnly {
		return marker.HiddenFromUser &&
			(marker.Synthetic || strings.TrimSpace(marker.Purpose) == "goal_continuation")
	}
	return !marker.HiddenFromUser
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

func transcriptRoundMarkerPresent(marker transcriptRoundMarker) bool {
	return strings.TrimSpace(marker.RoundID) != "" || strings.TrimSpace(marker.Content) != ""
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
