// INPUT: transcript 文件、读模型里上次发布的检查点、已冻结前缀的身份集合与当前 round marker。
// OUTPUT: 从检查点字节偏移续读完整行得到的尾部投影（与完整投影逐行一致）及下一个可持久化的检查点；无法证明前缀不变时报错。
// POS: DM transcript 投影位置的持久化协议；进程重启后也只解析新增字节，不依赖解析缓存。
package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 完整读取时只保留最后这么多个轮次起点作为检查点候选；尾部分组数有上限，更早的起点用不上。
const maxTranscriptCheckpointCandidates = 32

// transcriptCheckpoint 是 DM transcript 投影的持久化位置：某个用户轮次起点条目所在的行。
// 该行之前的条目与投影结果此后不再变化，续读从 Offset 开始重放该行及其后的完整行。
// JSON 形状存放在读模型里，改动需提升读模型版本。
type transcriptCheckpoint struct {
	// Line 与 Offset 是起点条目的 JSONL 行号和行首字节偏移，UUID 用于确认该行仍是同一条目。
	Line   int    `json:"line"`
	Offset int64  `json:"offset"`
	UUID   string `json:"uuid"`
	// State 是处理起点条目之前的投影状态。
	State transcriptProjectionState `json:"state"`
	// Turns 是起点之前主链上的全部用户轮次，marker 对齐是全局匹配，续读时需要它们重新对齐。
	Turns []transcriptUserTurn `json:"turns"`
	// AlignedTurns 是建立检查点时参与 marker 对齐的用户轮次总数（没有 marker 时为 0）；
	// MarkersDigest 是起点之前已消费的对齐 marker 的摘要，重新对齐后不一致说明前缀投影已变。
	AlignedTurns  int    `json:"aligned_turns"`
	MarkersDigest string `json:"markers_digest"`
}

// markersApply 判断按当前 marker 重新对齐后，起点之前的投影是否与建立检查点时一致。
// 已消费的 marker 数在对齐结果耗尽时会停止增长：建立时没有 marker 的检查点只在仍然没有 marker 时成立，
// 否则要求当时未耗尽（State.MarkerIndex 即起点之前真实消费的个数）且这些 marker 逐个未变。
func (c transcriptCheckpoint) markersApply(aligned []transcriptRoundMarker) bool {
	if c.AlignedTurns == 0 {
		return len(aligned) == 0
	}
	return c.State.MarkerIndex < c.AlignedTurns && c.State.MarkerIndex <= len(aligned) &&
		transcriptMarkersDigest(aligned[:c.State.MarkerIndex]) == c.MarkersDigest
}

// transcriptReplay 是一次 transcript 投影及其检查点候选。完整读取时 Rows 是全部投影行，
// 续读时 Rows 从 base 检查点的起点条目开始；Starts 的位置相对 chain、行数相对 Rows。
type transcriptReplay struct {
	Rows   []protocol.Message
	Starts []transcriptRoundStart

	base    transcriptCheckpoint
	chain   []transcriptEntry
	entries []transcriptEntry
	aligned []transcriptRoundMarker
	// offsets 与 Starts 一一对应，是各起点条目所在行的行首字节偏移。
	offsets []int64
}

// checkpoint 以第 index 个候选起点建立检查点，并返回因此新冻结的条目身份
// （上一个检查点到该起点之间的条目），由调用方并入已冻结前缀的身份集合。
func (r *transcriptReplay) checkpoint(index int) (transcriptCheckpoint, historyTailKeys) {
	start := r.Starts[index]
	entry := r.chain[start.Position]
	turns := append(
		slices.Clone(r.base.Turns),
		collectTranscriptUserTurnsAfter(r.chain[:start.Position], shouldSkipTranscriptEntry, r.base.State.LastTimestamp)...,
	)
	frozen := make(historyTailKeys)
	for _, item := range r.entries {
		if item.Index < entry.Index {
			frozen.addTranscriptEntry(item.Data)
		}
	}
	return transcriptCheckpoint{
		Line:          entry.Index,
		Offset:        r.offsets[index],
		UUID:          stringFromAny(entry.Data["uuid"]),
		State:         start.State,
		Turns:         turns,
		AlignedTurns:  len(r.aligned),
		MarkersDigest: transcriptMarkersDigest(r.aligned[:min(start.State.MarkerIndex, len(r.aligned))]),
	}, frozen
}

// newTranscriptReplay 为一次完整投影准备检查点候选。只有主链按行号严格递增时，
// “行号小于起点的条目”才恰好是起点之前的主链前缀，检查点才成立。
func newTranscriptReplay(
	ctx context.Context,
	file io.ReaderAt,
	size int64,
	entries []transcriptEntry,
	chain []transcriptEntry,
	projection transcriptProjection,
) (*transcriptReplay, error) {
	if len(projection.Starts) == 0 || !transcriptChainOrdered(chain) {
		return nil, nil
	}
	starts := projection.Starts[max(len(projection.Starts)-maxTranscriptCheckpointCandidates, 0):]
	offsets, err := transcriptLineOffsets(ctx, io.NewSectionReader(file, 0, size), 0, 0, transcriptRoundStartLines(chain, starts))
	if err != nil {
		return nil, err
	}
	if offsets == nil {
		return nil, nil
	}
	return &transcriptReplay{
		Rows:    projection.Rows,
		Starts:  starts,
		chain:   chain,
		entries: entries,
		aligned: projection.Aligned,
		offsets: offsets,
	}, nil
}

// resumeTranscriptProjection 从持久化检查点续读 transcript：只读取 current 快照范围内、
// 检查点偏移之后的完整行（末尾半行留给下一次），重建尾部主链并重放投影。
// 任何无法证明“完整投影 == 已冻结前缀 ++ 尾部投影”的情况都返回 errHistoryPageIndexInvalid。
// touchesFrozen 判断一组条目身份是否与已冻结前缀的身份相交。
func (s *AgentHistoryStore) resumeTranscriptProjection(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	previous historyPageSourceSnapshot,
	current historyPageSourceSnapshot,
	checkpoint transcriptCheckpoint,
	roundMarkers []transcriptRoundMarker,
	touchesFrozen func(historyTailKeys) (bool, error),
) (*transcriptReplay, error) {
	data, err := s.readTranscriptCheckpointTail(ctx, workspacePath, previous, current, checkpoint.Offset)
	if err != nil {
		return nil, err
	}
	entries, _, err := readTranscriptEntriesReaderContext(ctx, bytes.NewReader(data), checkpoint.Line)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || entries[0].Index != checkpoint.Line || stringFromAny(entries[0].Data["uuid"]) != checkpoint.UUID {
		return nil, fmt.Errorf("transcript checkpoint line changed: %w", errHistoryPageIndexInvalid)
	}
	// 尾部条目一旦与前缀共享 uuid、工具调用，或以前缀条目为父的并行工具结果，
	// 主链前缀的选择与并行条目归属就可能改变，只能完整重建。
	touched := make(historyTailKeys)
	for index, entry := range entries {
		touched.add(historyTailKeyTranscriptLink, stringFromAny(entry.Data["uuid"]))
		touched.addTranscriptTools(entry.Data)
		if index > 0 && len(transcriptToolResultIDs(entry.Data)) > 0 && !shouldSkipTranscriptEntry(entry.Data) {
			touched.add(historyTailKeyTranscriptLink, stringFromAny(entry.Data["parentUuid"]))
		}
	}
	if conflict, err := touchesFrozen(touched); err != nil {
		return nil, err
	} else if conflict {
		return nil, fmt.Errorf("transcript append touches earlier entries: %w", errHistoryPageIndexInvalid)
	}
	chain := buildPrimaryTranscriptChain(entries)
	if len(chain) == 0 || chain[0].Index != checkpoint.Line || !transcriptChainOrdered(chain) {
		return nil, fmt.Errorf("transcript chain left the checkpoint: %w", errHistoryPageIndexInvalid)
	}

	var aligned []transcriptRoundMarker
	if len(roundMarkers) > 0 {
		turns := append(
			slices.Clone(checkpoint.Turns),
			collectTranscriptUserTurnsAfter(chain, shouldSkipTranscriptEntry, checkpoint.State.LastTimestamp)...,
		)
		aligned = matchTranscriptRoundMarkers(turns, roundMarkers)
	}
	if !checkpoint.markersApply(aligned) {
		return nil, fmt.Errorf("round markers before the checkpoint changed: %w", errHistoryPageIndexInvalid)
	}

	state := checkpoint.State
	replay := &transcriptReplay{
		Rows:    make([]protocol.Message, 0, len(chain)),
		base:    checkpoint,
		chain:   chain,
		entries: entries,
		aligned: aligned,
	}
	visitProjectedTranscriptFrom(
		workspacePath,
		sessionValue.SessionKey,
		sessionValue.AgentID,
		slices.Values(chain),
		aligned,
		shouldSkipTranscriptEntry,
		&state,
		func(start transcriptRoundStart, _ transcriptProjectionState) {
			replay.Starts = append(replay.Starts, start)
		},
		func(rows ...protocol.Message) {
			replay.Rows = append(replay.Rows, rows...)
		},
	)
	// 起点条目必须再次重建 processor（例如它的 marker 没有变得不可物化），否则其后的条目
	// 会落到一个从未恢复的 processor 上。
	if len(replay.Starts) == 0 || replay.Starts[0].Position != 0 {
		return nil, fmt.Errorf("transcript checkpoint no longer starts a round: %w", errHistoryPageIndexInvalid)
	}
	replay.offsets, err = transcriptLineOffsets(
		ctx, bytes.NewReader(data), checkpoint.Line, checkpoint.Offset, transcriptRoundStartLines(chain, replay.Starts),
	)
	if err != nil || replay.offsets == nil {
		return nil, fmt.Errorf("transcript round start offsets unavailable: %w", errHistoryPageIndexInvalid)
	}
	return replay, nil
}

// readTranscriptCheckpointTail 校验 transcript 相对上次发布只是同一文件的纯追加，
// 并返回 [offset, current.Size) 内以换行结尾的完整行。
func (s *AgentHistoryStore) readTranscriptCheckpointTail(
	ctx context.Context,
	workspacePath string,
	previous historyPageSourceSnapshot,
	current historyPageSourceSnapshot,
	offset int64,
) ([]byte, error) {
	if !previous.Exists || !current.Exists || previous.FileIdentity == "" || previous.FileIdentity != current.FileIdentity ||
		current.Size < previous.Size || offset < 0 || offset >= previous.Size {
		return nil, fmt.Errorf("transcript replaced or truncated: %w", errHistoryPageIndexInvalid)
	}
	if current.Size-offset > historyPageSourceMaxBytes {
		return nil, fmt.Errorf("transcript append budget exceeded: %w", errHistoryPageIndexInvalid)
	}
	root, relative, info, err := s.openTranscriptPath(workspacePath, current.ResolvedPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := s.openTranscriptReadFile(root, relative)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	// 打开的必须正是 current 快照描述的文件，之后读到的字节才受下面的前缀校验保护。
	opened, err := snapshotOpenedFile(ctx, file, info, historyPageSourceSnapshot{
		Kind:          current.Kind,
		WorkspacePath: current.WorkspacePath,
		SessionID:     current.SessionID,
		ResolvedPath:  current.ResolvedPath,
	})
	if err != nil {
		return nil, err
	}
	if opened != current {
		return nil, fmt.Errorf("transcript changed during refresh: %w", errHistoryPageIndexInvalid)
	}
	if current != previous {
		fingerprint, err := historyPageFileEdgeFingerprint(ctx, file, previous.Size)
		if err != nil {
			return nil, err
		}
		if fingerprint != previous.EdgeFingerprint {
			return nil, fmt.Errorf("transcript prefix changed: %w", errHistoryPageIndexInvalid)
		}
	}
	data := make([]byte, current.Size-offset)
	if _, err = io.ReadFull(io.NewSectionReader(file, offset, int64(len(data))), data); err != nil {
		return nil, err
	}
	if offset > 0 {
		var boundary [1]byte
		if _, err = file.ReadAt(boundary[:], offset-1); err != nil || boundary[0] != '\n' {
			return nil, fmt.Errorf("transcript checkpoint is not at a line start: %w", errHistoryPageIndexInvalid)
		}
	}
	// 末尾没有换行的半行可能仍在写入，留给下一次续读。
	return data[:bytes.LastIndexByte(data, '\n')+1], ctx.Err()
}

// transcriptChainOrdered 判断主链是否按行号严格递增（父条目总在子条目之前落盘）。
func transcriptChainOrdered(chain []transcriptEntry) bool {
	for index := 1; index < len(chain); index++ {
		if chain[index].Index <= chain[index-1].Index {
			return false
		}
	}
	return true
}

func transcriptRoundStartLines(chain []transcriptEntry, starts []transcriptRoundStart) []int {
	lines := make([]int, len(starts))
	for index, start := range starts {
		lines[index] = chain[start.Position].Index
	}
	return lines
}

// transcriptLineOffsets 返回若干行（升序行号）的行首字节偏移。input 从第 firstLine 行的行首、
// 文件偏移 firstOffset 处开始；行号与解析器一致，按换行符计数。输入不足这些行时返回 nil。
func transcriptLineOffsets(ctx context.Context, input io.Reader, firstLine int, firstOffset int64, lines []int) ([]int64, error) {
	offsets := make([]int64, 0, len(lines))
	line, offset := firstLine, firstOffset
	buffer := make([]byte, transcriptReadBufferBytes)
	for len(offsets) < len(lines) {
		if lines[len(offsets)] == line {
			offsets = append(offsets, offset)
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		read, err := input.Read(buffer)
		for chunk := buffer[:read]; len(offsets) < len(lines); {
			newline := bytes.IndexByte(chunk, '\n')
			if newline < 0 {
				offset += int64(len(chunk))
				break
			}
			chunk = chunk[newline+1:]
			offset += int64(newline + 1)
			line++
			for len(offsets) < len(lines) && lines[len(offsets)] == line {
				offsets = append(offsets, offset)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if len(offsets) < len(lines) {
		return nil, nil
	}
	return offsets, nil
}

func transcriptMarkersDigest(markers []transcriptRoundMarker) string {
	hasher := sha256.New()
	encoder := json.NewEncoder(hasher)
	// marker 只含字符串、数值与按键排序的 map，编码结果确定；逐条编码避免为整段前缀分配缓冲。
	for index := range markers {
		_ = encoder.Encode(&markers[index])
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
