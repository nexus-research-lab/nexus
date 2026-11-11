// INPUT: 一个成员的受控 JSONL、round marker 和 Room 已公开引用的消息 ID。
// OUTPUT: 与主链投影相同的公开消息；正文按文件区间读取，内存只保留链结构和命中的消息。
// POS: 大 Room 的单成员回填，复用普通历史的分支选择、marker 对齐和 SDK 消息处理器。
package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"os"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type transcriptLineSpan struct {
	offset int64
	length int
}

type roomTranscriptReader struct {
	file  *os.File
	chain []transcriptEntry
	spans map[int]transcriptLineSpan
	turns map[int]transcriptUserTurn
	err   error
}

// 记录物理字节数，CRLF、空行和结尾没有换行的旧文件都能准确回读。
func scanTranscriptLine(data []byte, atEOF bool) (int, []byte, error) {
	if n := bytes.IndexByte(data, '\n'); n >= 0 {
		return n + 1, data[:n+1], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func newRoomTranscriptReader(ctx context.Context, file *os.File, collectTurns bool, headerBudget roomRefBudget) (*roomTranscriptReader, error) {
	reader := &roomTranscriptReader{file: file, spans: make(map[int]transcriptLineSpan), turns: make(map[int]transcriptUserTurn)}
	var entries []transcriptEntry
	var offset int64
	scanner := bufio.NewScanner(file)
	scanner.Split(scanTranscriptLine)
	scanner.Buffer(make([]byte, transcriptReadBufferBytes), transcriptScannerBufferBytes)
	for line := 0; scanner.Scan(); line++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data := scanner.Bytes()
		span := transcriptLineSpan{offset: offset, length: len(data)}
		offset += int64(len(data))
		var entry map[string]any
		if json.Unmarshal(bytes.TrimSpace(data), &entry) != nil || stringFromAny(entry["uuid"]) == "" {
			continue
		}
		header := roomTranscriptChainHeader(entry)
		encoded, err := json.Marshal(header)
		if err != nil {
			return nil, err
		}
		// 给每行的 map、文件位置和构建父链时的索引预留固定开销。
		weight := len(encoded) + 512
		if collectTurns {
			normalizeTranscriptEntryShape(entry)
			if turn, ok := transcriptUserTurnForMarker(entry, 0); ok {
				reader.turns[line] = turn
				weight += len(turn.Content)
			}
		}
		if err = headerBudget.replace(0, weight); err != nil {
			return nil, err
		}
		reader.spans[line] = span
		entries = append(entries, transcriptEntry{Index: line, Data: header})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	reader.chain = buildPrimaryTranscriptChain(entries)
	return reader, ctx.Err()
}

// 主链选择只依赖父子身份、可见性和并行工具配对。正文在真正投影该行时从同一个文件句柄读取。
func roomTranscriptChainHeader(entry map[string]any) map[string]any {
	header := map[string]any{
		"uuid": stringFromAny(entry["uuid"]), "parentUuid": stringFromAny(entry["parentUuid"]),
		"type": stringFromAny(entry["type"]), "timestamp": stringFromAny(entry["timestamp"]),
		"isMeta": shouldSkipTranscriptEntry(entry),
	}
	var blocks []map[string]any
	for _, id := range transcriptToolUseIDs(entry) {
		blocks = append(blocks, map[string]any{"type": "tool_use", "id": id})
	}
	for _, id := range transcriptToolResultIDs(entry) {
		blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": id})
	}
	if len(blocks) > 0 {
		header["message"] = map[string]any{"content": blocks}
	}
	if att, ok := entry["attachment"].(map[string]any); ok && stringFromAny(att["type"]) == "structured_output" {
		data, _ := att["data"].(map[string]any)
		header["attachment"] = map[string]any{"type": "structured_output", "data": map[string]any{
			"agent_id": stringFromAny(data["agent_id"]), "agentId": stringFromAny(data["agentId"]), "tool_use_id": stringFromAny(data["tool_use_id"]), "toolUseId": stringFromAny(data["toolUseId"]),
		}}
	}
	return header
}

func (r *roomTranscriptReader) alignedMarkers(markers []transcriptRoundMarker) []transcriptRoundMarker {
	if len(markers) == 0 {
		return nil
	}
	var turns []transcriptUserTurn
	var timestamp int64
	for _, entry := range r.chain {
		if shouldSkipTranscriptEntry(entry.Data) {
			continue
		}
		timestamp = transcriptEntryTimestamp(entry.Data, entry.Index, timestamp)
		if turn, ok := r.turns[entry.Index]; ok {
			if !turn.Empty {
				turn.Timestamp = timestamp
			}
			turns = append(turns, turn)
		}
	}
	return matchTranscriptRoundMarkers(turns, markers)
}

func (r *roomTranscriptReader) entries(ctx context.Context) iter.Seq[transcriptEntry] {
	return func(yield func(transcriptEntry) bool) {
		var buf []byte
		for _, entry := range r.chain {
			if r.err = ctx.Err(); r.err != nil {
				return
			}
			if !shouldSkipTranscriptEntry(entry.Data) {
				span := r.spans[entry.Index]
				if cap(buf) < span.length {
					buf = make([]byte, span.length)
				}
				buf = buf[:span.length]
				if _, r.err = r.file.ReadAt(buf, span.offset); r.err != nil {
					return
				}
				// 不得把完整内容解码到共享的 header map。
				entry.Data = nil
				if r.err = json.Unmarshal(bytes.TrimSpace(buf), &entry.Data); r.err != nil {
					return
				}
				normalizeTranscriptEntryShape(entry.Data)
			}
			if !yield(entry) || r.err != nil {
				return
			}
		}
	}
}

func (s *AgentHistoryStore) projectRoomTranscriptRefs(ctx context.Context, ref roomTranscriptRef, wanted map[string]bool, budget *roomRefBudget) (map[string]protocol.Message, error) {
	result := make(map[string]protocol.Message)
	path, err := s.resolveTranscriptPathContext(ctx, ref.WorkspacePath, ref.SessionID)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	root, name, _, err := s.openTranscriptPath(ref.WorkspacePath, path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := s.openTranscriptReadFile(root, name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	markers, err := s.readRoomRefRoundMarkers(ctx, ref)
	if err != nil {
		return nil, err
	}
	r, err := newRoomTranscriptReader(ctx, file, len(markers) > 0, *budget)
	if err != nil {
		return nil, err
	}
	aligned := r.alignedMarkers(markers)
	sizes := make(map[string]int)
	visitProjectedTranscript(ref.WorkspacePath, ref.PrivateSessionKey, ref.AgentID, r.entries(ctx), aligned, shouldSkipTranscriptEntry, func(rows ...protocol.Message) {
		for _, row := range rows {
			if id := stringFromAny(row["message_id"]); wanted[id] {
				mergeRoomTranscriptMessage(result, id, row)
				raw, err := marshalHistoryPageJSONBounded(result[id], int64(*budget)+int64(sizes[id]))
				if err != nil {
					r.err = err
					return
				}
				if err = budget.replace(sizes[id], len(raw)); err != nil {
					r.err = err
					return
				}
				sizes[id] = len(raw)
			}
		}
	})
	return result, r.err
}

func (s *AgentHistoryStore) readRoomRefRoundMarkers(ctx context.Context, ref roomTranscriptRef) ([]transcriptRoundMarker, error) {
	path := s.paths.SessionOverlayPath(ref.WorkspacePath, ref.PrivateSessionKey)
	root, name, err := s.files.openOwnerWorkspaceFileParent(s.ownerUserID, ref.WorkspacePath, path, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.OpenFileNoSymlink(name, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readTranscriptRoundMarkers(ctx, file)
}

func readTranscriptRoundMarkers(ctx context.Context, file io.Reader) ([]transcriptRoundMarker, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, transcriptReadBufferBytes), transcriptScannerBufferBytes)
	var rows []map[string]any
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var header struct {
			Kind string `json:"nexus_overlay_kind"`
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if json.Unmarshal(line, &header) != nil || stringFromAny(header.Kind) != overlayKindRoundMarker {
			continue
		}
		var row map[string]any
		if json.Unmarshal(line, &row) == nil {
			rows = append(rows, normalizeDecodedJSONValue(row).(map[string]any))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return overlayHistoryStateFromRows(rows).RoundMarkers, ctx.Err()
}
