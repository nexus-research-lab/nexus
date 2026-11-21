// INPUT: 已筛选 transcript 链、durable round marker 与用户输入时间。
// OUTPUT: 与正式历史投影一一对应、识别空白 Goal 续跑与 runtime 自唤醒、且不会跨可见性错绑的 round marker。
// POS: transcript 用户轮次与 Nexus durable round 身份的唯一对齐边界。
package workspace

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/message"
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
	return matchTranscriptRoundMarkers(collectTranscriptUserTurns(chain, shouldSkip), roundMarkers)
}

func matchTranscriptRoundMarkers(userTurns []transcriptUserTurn, roundMarkers []transcriptRoundMarker) []transcriptRoundMarker {
	if len(userTurns) == 0 {
		return nil
	}

	aligned := make([]transcriptRoundMarker, len(userTurns))
	used := make([]bool, len(roundMarkers))
	// 按清洗后的内容预建候选表：长会话中每个用户轮次只比较同内容的 marker，
	// 避免 turns×markers 次重复清洗。候选按原下标升序，选择规则不变。
	candidates := make(map[string][]int, len(roundMarkers))
	for index, marker := range roundMarkers {
		content := transcriptUserContentKey(sanitizeTranscriptUserContent(marker.Content))
		candidates[content] = append(candidates[content], index)
	}
	for index, turn := range userTurns {
		markerIndex := findMatchingRoundMarker(roundMarkers, candidates[turn.Content], used, turn)
		if markerIndex < 0 {
			continue
		}
		aligned[index] = roundMarkers[markerIndex]
		used[markerIndex] = true
	}

	for index, turn := range userTurns {
		if turn.Empty || transcriptRoundMarkerPresent(aligned[index]) {
			continue
		}
		markerIndex := findCompatibleFallbackRoundMarker(roundMarkers, used, turn)
		if markerIndex < 0 {
			continue
		}
		aligned[index] = roundMarkers[markerIndex]
		used[markerIndex] = true
	}
	// nxs 在 user 落盘前提取隐藏 reminder，Goal-only 输入因此可能是空串。
	// 先保留所有显式输入的匹配，再仅用隐藏 Goal marker 恢复这类边界。
	for index, turn := range userTurns {
		if !turn.Empty {
			continue
		}
		markerIndex := findCompatibleFallbackRoundMarker(roundMarkers, used, turn)
		if markerIndex < 0 {
			continue
		}
		marker := roundMarkers[markerIndex]
		distance, _ := transcriptRoundMarkerDistance(turn.Timestamp, marker.Timestamp)
		uniqueClosest := true
		for otherIndex, other := range userTurns {
			if otherIndex == index || !other.Empty || transcriptRoundMarkerPresent(aligned[otherIndex]) ||
				!roundMarkerFallbackCompatible(other, marker) {
				continue
			}
			if otherDistance, ok := transcriptRoundMarkerDistance(other.Timestamp, marker.Timestamp); ok && otherDistance <= distance {
				uniqueClosest = false
				break
			}
		}
		if uniqueClosest {
			aligned[index] = marker
			used[markerIndex] = true
		}
	}
	return aligned
}

// transcriptUserTurn 是 marker 对齐所需的用户轮次摘要。JSON 形状随读模型检查点持久化，
// 改动需提升读模型版本。
type transcriptUserTurn struct {
	// Content 是清洗后内容的摘要键（见 transcriptUserContentKey），只用于与 marker 内容做等值匹配。
	Content         string `json:"content,omitempty"`
	Timestamp       int64  `json:"timestamp,omitempty"`
	GoalContextOnly bool   `json:"goal_context_only,omitempty"`
	Empty           bool   `json:"empty,omitempty"`
	// RuntimeWake 表示 runtime 自唤醒注入的输入，只能与自唤醒 marker 互相对齐。
	RuntimeWake bool `json:"runtime_wake,omitempty"`
}

// transcriptUserContentKey 把清洗后的用户内容压成定长摘要：检查点要保存全部历史轮次，
// 不能随用户输入的长度增长。空内容保持为空，对齐规则据此跳过内容匹配。
func transcriptUserContentKey(content string) string {
	if content == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(content))
	return base64.RawStdEncoding.EncodeToString(sum[:16])
}

func collectTranscriptUserTurns(
	chain []transcriptEntry,
	shouldSkip func(map[string]any) bool,
) []transcriptUserTurn {
	return collectTranscriptUserTurnsAfter(chain, shouldSkip, 0)
}

// collectTranscriptUserTurnsAfter 收集主链片段中的用户轮次；lastTimestamp 是片段之前
// 最后一个条目的时间，与投影循环的 LastTimestamp 一致，保证从中途续接时缺省时间相同。
func collectTranscriptUserTurnsAfter(
	chain []transcriptEntry,
	shouldSkip func(map[string]any) bool,
	lastTimestamp int64,
) []transcriptUserTurn {
	turns := make([]transcriptUserTurn, 0)
	for _, entry := range chain {
		if shouldSkip != nil && shouldSkip(entry.Data) {
			continue
		}
		entryTimestamp := transcriptEntryTimestamp(entry.Data, entry.Index, lastTimestamp)
		lastTimestamp = entryTimestamp
		if turn, ok := transcriptUserTurnForMarker(entry.Data, entryTimestamp); ok {
			turns = append(turns, turn)
		}
	}
	return turns
}

func transcriptUserTurnForMarker(data map[string]any, timestamp int64) (transcriptUserTurn, bool) {
	if stringFromAny(data["type"]) != "user" || len(transcriptToolResultIDs(data)) != 0 {
		return transcriptUserTurn{}, false
	}
	decoded, err := sdkprotocol.DecodeMessage(data)
	if err != nil || decoded.Type != sdkprotocol.MessageTypeUser || isTranscriptToolResult(decoded) || !shouldAlignTranscriptUserTurn(data) {
		return transcriptUserTurn{}, false
	}
	empty := isEmptyTranscriptUserTurn(data)
	if empty {
		timestamp = transcriptEntryTimestamp(data, -1, 0)
	}
	_, runtimeWake := transcriptRuntimeWakeTrigger(data, transcriptRoundMarker{})
	return transcriptUserTurn{
		Content:         transcriptUserContentKey(transcriptUserContent(data)),
		Timestamp:       timestamp,
		GoalContextOnly: isTranscriptGoalContextOnlyUserTurn(data),
		Empty:           empty,
		RuntimeWake:     runtimeWake,
	}, true
}

// findMatchingRoundMarker 在 candidates（内容与 turn 相同、按下标升序的 marker）中选时间最近者。
func findMatchingRoundMarker(
	roundMarkers []transcriptRoundMarker,
	candidates []int,
	used []bool,
	turn transcriptUserTurn,
) int {
	if turn.Content == "" {
		return -1
	}
	bestIndex := -1
	var bestDistance int64
	for _, index := range candidates {
		marker := roundMarkers[index]
		if index < len(used) && used[index] {
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
	ambiguous := false
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
			ambiguous = turn.Empty && bestIndex >= 0 && distance == bestDistance
			bestIndex = index
			bestDistance = distance
		}
	}
	if ambiguous {
		return -1
	}
	return bestIndex
}

func roundMarkerFallbackCompatible(turn transcriptUserTurn, marker transcriptRoundMarker) bool {
	if turn.RuntimeWake || isRuntimeWakeRoundMarker(marker) {
		return turn.RuntimeWake && isRuntimeWakeRoundMarker(marker)
	}
	if turn.Empty {
		return turn.Timestamp > 0 && marker.Timestamp > 0 && marker.HiddenFromUser &&
			strings.TrimSpace(marker.Purpose) == "goal_continuation" && strings.TrimSpace(marker.Content) == ""
	}
	if turn.GoalContextOnly {
		return marker.HiddenFromUser &&
			(marker.Synthetic || strings.TrimSpace(marker.Purpose) == "goal_continuation")
	}
	return !marker.HiddenFromUser
}

func isRuntimeWakeRoundMarker(marker transcriptRoundMarker) bool {
	return strings.TrimSpace(marker.Purpose) == message.RuntimeWakePurpose || message.IsRuntimeWakeMetadata(marker.Metadata)
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
	if isTranscriptLocalCommandResultUserTurn(entry) {
		return false
	}
	return transcriptUserContent(entry) != "" ||
		isTranscriptGoalContextOnlyUserTurn(entry) ||
		message.IsInternalExplicitSkillPrompt(transcriptRawUserContent(entry))
}

func shouldAlignTranscriptUserTurn(entry map[string]any) bool {
	return shouldMaterializeTranscriptUserTurn(entry) || isEmptyTranscriptUserTurn(entry)
}

func isEmptyTranscriptUserTurn(entry map[string]any) bool {
	messageValue, _ := entry["message"].(map[string]any)
	switch content := messageValue["content"].(type) {
	case string:
		return strings.TrimSpace(content) == ""
	case []any:
		return len(content) == 0
	default:
		return false
	}
}

func isTranscriptGoalContextOnlyUserTurn(entry map[string]any) bool {
	content := strings.TrimSpace(transcriptRawUserContent(entry))
	if strings.HasPrefix(content, "<goal_context>") &&
		strings.HasSuffix(content, "</goal_context>") {
		return true
	}
	remaining, stripped, hasGoal := stripLeadingTranscriptInternalContexts(content)
	return stripped && hasGoal && strings.TrimSpace(remaining) == ""
}

func consumeTranscriptRoundMarker(markers []transcriptRoundMarker, index *int) transcriptRoundMarker {
	if index == nil || *index >= len(markers) {
		return transcriptRoundMarker{}
	}
	marker := markers[*index]
	*index++
	return marker
}
