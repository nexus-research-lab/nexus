package workspace

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/message"
)

func alignTranscriptRoundMarkers(
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
		if !isEmptyTranscriptRoundMarker(aligned[index]) {
			continue
		}
		markerIndex := findNearestRoundMarker(roundMarkers, used, turn)
		if markerIndex < 0 {
			continue
		}
		aligned[index] = roundMarkers[markerIndex]
		used[markerIndex] = true
	}
	return aligned
}

type transcriptUserTurn struct {
	Content     string
	GoalCarrier bool
	Timestamp   int64
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
			goalCarrier := isTranscriptGoalContextOnlyUserTurn(entry.Data)
			turns = append(turns, transcriptUserTurn{
				Content:     transcriptUserContent(entry.Data),
				GoalCarrier: goalCarrier,
				Timestamp:   entryTimestamp,
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
		return findGoalRoundMarker(roundMarkers, used, turn)
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
	if bestIndex < 0 {
		return findGoalRoundMarker(roundMarkers, used, turn)
	}
	return bestIndex
}

func findNearestRoundMarker(
	roundMarkers []transcriptRoundMarker,
	used []bool,
	turn transcriptUserTurn,
) int {
	if turn.Timestamp <= 0 {
		return findGoalRoundMarker(roundMarkers, used, turn)
	}
	// 中文注释：Room 等宿主流程会把可见输入包装成 runtime prompt，
	// 内容无法精确相等时只允许在短时间窗内一对一回退，避免旧 turn 抢走新 marker。
	const markerFallbackToleranceMS = 30 * 1000
	bestIndex := -1
	var bestDistance int64
	for index, marker := range roundMarkers {
		if index < len(used) && used[index] {
			continue
		}
		distance, ok := transcriptRoundMarkerDistance(turn.Timestamp, marker.Timestamp)
		if !ok || distance > markerFallbackToleranceMS {
			continue
		}
		if bestIndex < 0 || distance < bestDistance || (distance == bestDistance && index > bestIndex) {
			bestIndex = index
			bestDistance = distance
		}
	}
	if bestIndex < 0 {
		return findGoalRoundMarker(roundMarkers, used, turn)
	}
	return bestIndex
}

func findGoalRoundMarker(
	roundMarkers []transcriptRoundMarker,
	used []bool,
	turn transcriptUserTurn,
) int {
	if !turn.GoalCarrier {
		return -1
	}
	for index, marker := range roundMarkers {
		if index < len(used) && used[index] {
			continue
		}
		if marker.HiddenFromUser &&
			marker.Synthetic &&
			strings.Contains(strings.ToLower(marker.Purpose), "goal") {
			return index
		}
	}
	return -1
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

func isEmptyTranscriptRoundMarker(marker transcriptRoundMarker) bool {
	return strings.TrimSpace(marker.RoundID) == "" &&
		strings.TrimSpace(marker.Content) == ""
}

func shouldMaterializeTranscriptUserTurn(entry map[string]any) bool {
	if isTranscriptLocalCommandResultUserTurn(entry) {
		return false
	}
	return transcriptUserContent(entry) != "" ||
		message.IsInternalExplicitSkillPrompt(transcriptRawUserContent(entry))
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
	if index == nil || *index >= len(markers) {
		return transcriptRoundMarker{}
	}
	marker := markers[*index]
	*index++
	return marker
}
