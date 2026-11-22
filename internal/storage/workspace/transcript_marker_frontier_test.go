// INPUT: 随机生成、逐步追加的用户轮次与 round marker（重复内容、隐藏 Goal marker、空白轮次、自唤醒、缺失或抖动的时间），以及间隔很短的连续轮次。
// OUTPUT: 从对齐边界续接的结果与对整条主链重新对齐逐项一致；续接成立时，已并入前缀的对齐没有变化；轮次只在最新 marker 够不着之后定型。
// POS: marker 增量对齐与全局对齐等价性的回归。
package workspace

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/message"
)

func TestTranscriptMarkerFrontierMatchesGlobalAlignment(t *testing.T) {
	random := rand.New(rand.NewPCG(337, 353))
	resumed, rejected := 0, 0
	for scenario := range 4000 {
		session := markerFrontierSession{random: random, clock: 1_700_000_000_000}
		var frontier transcriptMarkerFrontier
		tracked, prefix := false, 0
		var frozen []transcriptMarkerMatch
		for step := range 10 {
			session.append()
			var global []transcriptMarkerMatch
			if len(session.markers) > 0 {
				global = matchTranscriptRoundMarkerIndexes(session.turns, session.markers)
			}
			alignment, base := alignTranscriptUserTurns(session.turns, session.markers), 0
			if len(session.markers) == 0 {
				alignment = transcriptMarkerAlignment{}
			}
			if tracked {
				if next, ok := frontier.resume(session.turns[prefix:], session.markers); ok {
					resumed++
					if !slices.Equal(next.matches[min(len(frontier.OpenTurns), len(next.matches)):], global[min(prefix, len(global)):]) ||
						!slices.Equal(global[:min(prefix, len(global))], frozen) {
						t.Fatalf("scenario %d step %d: resumed %+v after prefix %+v, global %+v\nturns %+v\nmarkers %+v",
							scenario, step, next.matches, frozen, global, session.turns, session.markers)
					}
					alignment, base = next, prefix
				} else {
					rejected++
				}
			}
			prefix = base + random.IntN(len(session.turns)-base+1)
			frontier, tracked = alignment.frontier(prefix - base)
			frozen = slices.Clone(global[:min(prefix, len(global))])
		}
	}
	// 两条分支都要有足够的样本，否则等价性断言形同虚设。
	if resumed < 10000 || rejected < 1000 {
		t.Fatalf("resumed %d, rejected %d", resumed, rejected)
	}
}

func TestTranscriptMarkerFrontierSealsTurnsOutOfReach(t *testing.T) {
	var turns []transcriptUserTurn
	var markers []transcriptRoundMarker
	for index, at := range []int64{100_000, 101_000, 111_000} {
		content := fmt.Sprintf("question %d", index)
		turns = append(turns, transcriptUserTurn{Content: transcriptUserContentKey(content), Timestamp: at})
		markers = append(markers, transcriptRoundMarker{RoundID: fmt.Sprintf("round-%d", index), Content: content, Timestamp: at})
	}
	// 前两轮只隔一秒：最新的 marker 还够得着第一轮，它先留作未定型。
	first, ok := alignTranscriptUserTurns(turns[:2], markers[:2]).frontier(1)
	if !ok || !reflect.DeepEqual(first, transcriptMarkerFrontier{
		Markers: 2, OpenTurns: []transcriptOpenTurn{{Turn: turns[0], Marker: 0}}, OpenMarkers: []int{0, 1},
	}) {
		t.Fatalf("first frontier = %+v, %v", first, ok)
	}
	// 十秒后的 marker 够不着前两轮，它们一起定型，检查点不再保存它们。
	resumed, ok := first.resume(turns[1:], markers)
	if !ok || !reflect.DeepEqual(resumed.aligned(), markers[1:]) {
		t.Fatalf("resumed alignment = %+v, %v", resumed.aligned(), ok)
	}
	second, ok := resumed.frontier(1)
	if !ok || !reflect.DeepEqual(second, transcriptMarkerFrontier{Markers: 3, SealedTimestamp: 101_000, OpenMarkers: []int{2}}) {
		t.Fatalf("second frontier = %+v, %v", second, ok)
	}
	// 时间乱序的新 marker 够得着已定型的轮次，它们的对齐可能改变：拒绝续接。
	late := append(slices.Clone(markers), transcriptRoundMarker{RoundID: "round-late", Content: "question 1", Timestamp: 101_500})
	if _, ok = second.resume(turns[2:], late); ok {
		t.Fatal("resumed although a new marker can reach a sealed turn")
	}
}

type markerFrontierSession struct {
	random  *rand.Rand
	clock   int64
	turns   []transcriptUserTurn
	markers []transcriptRoundMarker
}

// append 推进时钟并追加若干 marker 与用户轮次；marker 通常先于对应的 transcript 用户条目落盘。
func (s *markerFrontierSession) append() {
	for range s.random.IntN(3) {
		s.clock += []int64{0, 1_000, 4_000, 6_000, 20_000, 15 * 60_000}[s.random.IntN(6)]
		content := []string{"a", "b", "c"}[s.random.IntN(3)]
		marker := transcriptRoundMarker{RoundID: fmt.Sprintf("round-%d", len(s.markers)), Content: content, Timestamp: s.timestamp()}
		turn := transcriptUserTurn{Content: transcriptUserContentKey(content), Timestamp: s.timestamp()}
		switch s.random.IntN(10) {
		case 0:
			marker.Content, marker.HiddenFromUser, marker.Purpose = "", true, "goal_continuation"
			turn.Content, turn.Empty = "", true
		case 1:
			marker.Content, marker.HiddenFromUser, marker.Synthetic = "", true, true
			turn.Content, turn.GoalContextOnly = "", true
		case 2:
			marker.Purpose, turn.RuntimeWake = message.RuntimeWakePurpose, true
		case 3:
			// transcript 里的内容与 marker 不同，只能靠回退匹配。
			turn.Content = transcriptUserContentKey(content + " edited")
		}
		if s.random.IntN(5) > 0 {
			s.markers = append(s.markers, marker)
		}
		if s.random.IntN(5) > 0 {
			s.turns = append(s.turns, turn)
		}
	}
}

// timestamp 返回带落盘抖动的当前时间，偶尔缺失。
func (s *markerFrontierSession) timestamp() int64 {
	if s.random.IntN(25) == 0 {
		return 0
	}
	return s.clock + int64(s.random.IntN(7_000)) - 3_000
}
