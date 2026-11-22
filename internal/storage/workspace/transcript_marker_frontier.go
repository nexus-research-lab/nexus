// INPUT: 检查点起点之前仍未定型的 marker 对齐、起点及其后的用户轮次与当前全部 round marker。
// OUTPUT: 只重新对齐未定型的轮次与 marker 得到的、与整条主链全局对齐逐项一致的结果，以及下一个可持久化的对齐边界；无法证明前缀对齐不变时报告失败。
// POS: DM transcript 检查点续接 marker 对齐的唯一入口，使每次续读的对齐成本不随历史轮次数增长。
package workspace

// transcriptMarkerFrontier 是检查点起点之前的 marker 对齐里仍会参与后续对齐的部分。
//
// 起点之前按内容等值匹配、且当时最新的 marker 在时间上已经够不着的轮次（下称定型轮次）及其占用的 marker 不保存：
// 等值匹配按轮次顺序进行，定型轮次的选择只取决于更早的轮次和内容相同的 marker，回退匹配既不会再处理它，
// 也拿不到它占用的 marker。因此只要之后追加的 marker 在时间上都够不着定型轮次、其余轮次重新对齐后也没有换 marker，
// 定型轮次的对齐就不变，其余轮次和 marker 与新增部分一起重新对齐的结果与对整条主链重新对齐一致。
// JSON 形状随读模型检查点持久化，改动需提升读模型版本。
type transcriptMarkerFrontier struct {
	// Markers 是建立时的 marker 总数；为 0 表示投影没有消费过 marker，只在仍然没有 marker 时成立。
	Markers int `json:"markers"`
	// SealedTimestamp 是定型轮次的最大时间，没有定型轮次时为 0。
	SealedTimestamp int64 `json:"sealed_timestamp"`
	// OpenTurns 是起点之前没有定型的用户轮次：未对齐、由回退匹配对齐，或等值匹配但最新的 marker 还够得着。
	OpenTurns []transcriptOpenTurn `json:"open_turns"`
	// OpenMarkers 是没有被定型轮次占用的 marker 下标，升序且都小于 Markers。
	OpenMarkers []int `json:"open_markers"`
}

// transcriptOpenTurn 是一个未定型的用户轮次；Marker 是它对齐到的 marker 下标，未对齐为 -1。
// 起点之前的投影已经按这个结果产出，续接时重新对齐的结果必须与它相同。
type transcriptOpenTurn struct {
	Turn   transcriptUserTurn `json:"turn"`
	Marker int                `json:"marker"`
}

// transcriptMarkerAlignment 是一次 marker 对齐的结果：base 留下的未定型轮次加上其后的用户轮次，与当前全部 marker 对齐。
type transcriptMarkerAlignment struct {
	base    transcriptMarkerFrontier
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

// resume 把起点及其后的用户轮次与当前 marker 对齐。前缀对齐可能已变（marker 减少、
// 新 marker 够得着定型轮次、未定型轮次换了 marker）时返回 false。
func (f transcriptMarkerFrontier) resume(
	turns []transcriptUserTurn,
	roundMarkers []transcriptRoundMarker,
) (transcriptMarkerAlignment, bool) {
	alignment := transcriptMarkerAlignment{base: f, markers: roundMarkers}
	if f.Markers == 0 {
		return alignment, len(roundMarkers) == 0
	}
	if len(roundMarkers) < f.Markers {
		return alignment, false
	}
	if f.SealedTimestamp > 0 {
		for _, marker := range roundMarkers[f.Markers:] {
			if _, reachable := transcriptRoundMarkerDistance(f.SealedTimestamp, marker.Timestamp); reachable {
				return alignment, false
			}
		}
	}
	positions := alignment.openMarkerPositions()
	open := make([]transcriptRoundMarker, len(positions))
	for index, position := range positions {
		if position < 0 || position >= len(roundMarkers) {
			return alignment, false
		}
		open[index] = roundMarkers[position]
	}
	alignment.turns = make([]transcriptUserTurn, 0, len(f.OpenTurns)+len(turns))
	for _, turn := range f.OpenTurns {
		alignment.turns = append(alignment.turns, turn.Turn)
	}
	alignment.turns = append(alignment.turns, turns...)
	alignment.matches = matchTranscriptRoundMarkerIndexes(alignment.turns, open)
	for index := range alignment.matches {
		if match := &alignment.matches[index]; match.Index >= 0 {
			match.Index = positions[match.Index]
		}
	}
	for index, turn := range f.OpenTurns {
		if alignment.matches[index].Index != turn.Marker {
			return alignment, false
		}
	}
	return alignment, true
}

// openMarkerPositions 返回本次参与对齐的 marker 下标：base 留下的未占用 marker 加上此后新增的 marker。
func (a transcriptMarkerAlignment) openMarkerPositions() []int {
	positions := make([]int, 0, len(a.base.OpenMarkers)+len(a.markers)-a.base.Markers)
	positions = append(positions, a.base.OpenMarkers...)
	for position := a.base.Markers; position < len(a.markers); position++ {
		positions = append(positions, position)
	}
	return positions
}

// aligned 返回 base 之后各用户轮次对齐到的 marker，供投影按轮次顺序消费；没有 marker 时为 nil。
func (a transcriptMarkerAlignment) aligned() []transcriptRoundMarker {
	return alignedTranscriptRoundMarkers(a.markers, a.matches[min(len(a.base.OpenTurns), len(a.matches)):])
}

// frontier 把 base 之后的前 count 个轮次并入前缀，返回下一个检查点的对齐边界。
// 等值匹配的轮次要等最新的 marker 在时间上够不着它才定型：新 marker 的时间通常不早于最新的 marker，
// 续接时的可达性检查因此只在 marker 时间乱序时失败。
func (a transcriptMarkerAlignment) frontier(count int) (transcriptMarkerFrontier, bool) {
	if len(a.markers) == 0 {
		return transcriptMarkerFrontier{}, true
	}
	count += len(a.base.OpenTurns)
	if count > len(a.matches) {
		return transcriptMarkerFrontier{}, false
	}
	next := transcriptMarkerFrontier{Markers: len(a.markers), SealedTimestamp: a.base.SealedTimestamp}
	latest := a.markers[len(a.markers)-1].Timestamp
	sealed := make(map[int]struct{}, count)
	for index, turn := range a.turns[:count] {
		match := a.matches[index]
		// 空白轮次在等值匹配之后还会被再匹配一次，Marker 记不下它在等值匹配阶段占用的 marker。
		if match.Exact && turn.Empty {
			return transcriptMarkerFrontier{}, false
		}
		if _, reachable := transcriptRoundMarkerDistance(turn.Timestamp, latest); !match.Exact || reachable {
			next.OpenTurns = append(next.OpenTurns, transcriptOpenTurn{Turn: turn, Marker: match.Index})
			continue
		}
		sealed[match.Index] = struct{}{}
		next.SealedTimestamp = max(next.SealedTimestamp, turn.Timestamp)
	}
	for _, position := range a.openMarkerPositions() {
		if _, taken := sealed[position]; !taken {
			next.OpenMarkers = append(next.OpenMarkers, position)
		}
	}
	return next, true
}
