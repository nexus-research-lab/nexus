// INPUT: 主链上的用户轮次与当前全部 round marker。
// OUTPUT: 各用户轮次对齐到的 marker，供投影按轮次顺序消费。
// POS: transcript 用户轮次与 Nexus round marker 的对齐结果（对齐规则见 transcript_marker.go）。
package workspace

// transcriptMarkerAlignment 是一次 marker 对齐的结果。
type transcriptMarkerAlignment struct {
	markers []transcriptRoundMarker
	turns   []transcriptUserTurn
	// matches 与 turns 一一对应，Index 是 marker 在 markers 中的下标；没有 marker 时为空。
	matches []transcriptMarkerMatch
}

// alignTranscriptUserTurns 对齐整条主链的用户轮次。
func alignTranscriptUserTurns(turns []transcriptUserTurn, roundMarkers []transcriptRoundMarker) transcriptMarkerAlignment {
	return transcriptMarkerAlignment{
		markers: roundMarkers,
		turns:   turns,
		matches: matchTranscriptRoundMarkerIndexes(turns, roundMarkers),
	}
}

// aligned 返回各用户轮次对齐到的 marker，供投影按轮次顺序消费；没有 marker 时为 nil。
func (a transcriptMarkerAlignment) aligned() []transcriptRoundMarker {
	return alignedTranscriptRoundMarkers(a.markers, a.matches)
}
