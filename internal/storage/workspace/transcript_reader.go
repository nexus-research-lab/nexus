// INPUT: runtime transcript JSONL 与父链/并行工具关联。
// OUTPUT: 供历史投影消费的主链（经 logicalParentUuid 跨 compact_boundary 续接，压缩摘要视为 meta），以及与主链工具调用对应的并行结果和子任务附件。
// POS: transcript 读取、规范化与执行链选择边界；完整解析经 transcript_parallel_read.go 分块并行。
package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/message"
)

func (s *AgentHistoryStore) readTranscriptEntriesAt(root *confinedfs.Root, relative string) ([]transcriptEntry, error) {
	return s.readTranscriptEntriesAtContext(context.Background(), root, relative)
}

func (s *AgentHistoryStore) readTranscriptEntriesAtContext(
	ctx context.Context,
	root *confinedfs.Root,
	relative string,
) ([]transcriptEntry, error) {
	entries, _, _, err := s.readTranscriptEntriesLineageAtContext(ctx, root, relative)
	return entries, err
}

// readTranscriptEntriesLineageAtContext 额外返回缓存路径与解析代系；
// 未进入缓存（读取期间仍在写入或文件过大）时代系为 0。
func (s *AgentHistoryStore) readTranscriptEntriesLineageAtContext(
	ctx context.Context,
	root *confinedfs.Root,
	relative string,
) ([]transcriptEntry, string, uint64, error) {
	file, err := s.openTranscriptReadFile(root, relative)
	if err != nil {
		return nil, "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", 0, err
	}
	before, err := snapshotOpenedFile(ctx, file, info, historyPageSourceSnapshot{})
	if err != nil {
		return nil, "", 0, err
	}
	path := filepath.Join(root.Name(), relative)
	if cached, lineage, ok := s.readTranscriptCacheLineage(path, before); ok {
		return cached, path, lineage, nil
	}
	if extended, lineage, ok, err := s.extendTranscriptCache(ctx, path, file, before); err != nil {
		return nil, "", 0, err
	} else if ok {
		return extended, path, lineage, nil
	}
	entries, nextIndex, err := readTranscriptEntriesParallelContext(ctx, file, before.Size)
	if err != nil {
		return nil, "", 0, err
	}
	info, err = file.Stat()
	if err != nil {
		return nil, "", 0, err
	}
	after, err := snapshotOpenedFile(ctx, file, info, historyPageSourceSnapshot{})
	if err != nil {
		return nil, "", 0, err
	}
	// 读取期间仍在写入的文件不进入缓存，由调用方原有 source 校验决定是否重试。
	var lineage uint64
	if before == after {
		lineage = s.writeTranscriptCache(path, after, entries, nextIndex)
	}
	return entries, path, lineage, nil
}

// openTranscriptReadFile 让 Room 引用投影和分页指纹也复用 owner ACL 修复。
// 重试仍使用已校验的目录句柄，不放宽路径或符号链接边界。
func (s *AgentHistoryStore) openTranscriptReadFile(root *confinedfs.Root, relative string) (*os.File, error) {
	return withRuntimePermissionRepair(s, func() (*os.File, error) {
		return root.OpenFileNoSymlink(relative, os.O_RDONLY, 0)
	})
}

func readTranscriptEntriesFileContext(ctx context.Context, file *os.File) ([]transcriptEntry, error) {
	entries, _, err := readTranscriptEntriesReaderContext(ctx, file, 0)
	return entries, err
}

// readTranscriptEntriesReaderContext 从 startIndex 开始按行编号解析 JSONL，
// 返回条目和下一个行号；空行与无法解析的行也占用行号，保证追加解析与完整解析编号一致。
func readTranscriptEntriesReaderContext(ctx context.Context, input io.Reader, startIndex int) ([]transcriptEntry, int, error) {
	reader := bufio.NewScanner(input)
	reader.Buffer(make([]byte, 0, transcriptReadBufferBytes), transcriptScannerBufferBytes)

	results := make([]transcriptEntry, 0)
	index := startIndex
	for ; reader.Scan(); index++ {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		// Unmarshal 不保留输入，直接解析扫描器缓冲区，免去每行两次复制。
		line := bytes.TrimSpace(reader.Bytes())
		if len(line) == 0 {
			continue
		}
		entry := map[string]any{}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		normalizeTranscriptEntryShape(entry)
		if stringFromAny(entry["uuid"]) == "" {
			continue
		}
		results = append(results, transcriptEntry{
			Index: index,
			Data:  entry,
		})
	}
	if err := reader.Err(); err != nil {
		return nil, 0, err
	}
	return results, index, ctx.Err()
}

func normalizeTranscriptEntryShape(entry map[string]any) {
	// 旧版 transcript 可能使用 camelCase 字段，这里统一转成 SDK 解码需要的 snake_case。
	if entry["session_id"] == nil && entry["sessionId"] != nil {
		entry["session_id"] = entry["sessionId"]
	}

	if stringFromAny(entry["type"]) != "assistant" {
		return
	}
	messageValue, ok := entry["message"].(map[string]any)
	if !ok {
		return
	}
	if stringFromAny(messageValue["id"]) != "" {
		return
	}
	if uuid := stringFromAny(entry["uuid"]); uuid != "" {
		messageValue["id"] = uuid
	}
}

func buildPrimaryTranscriptChain(entries []transcriptEntry) []transcriptEntry {
	return buildTranscriptChain(entries, shouldSkipTranscriptEntry)
}

func buildExplicitTranscriptChain(entries []transcriptEntry) []transcriptEntry {
	return buildTranscriptChain(entries, shouldSkipExplicitTranscriptEntry)
}

func buildTranscriptChain(
	entries []transcriptEntry,
	shouldSkip func(map[string]any) bool,
) []transcriptEntry {
	if len(entries) == 0 {
		return nil
	}
	byUUID, parentUUIDs := indexTranscriptEntries(entries)
	terminals := transcriptTerminalEntries(byUUID, parentUUIDs, shouldSkip)
	if len(terminals) == 0 {
		return nil
	}
	sort.Slice(terminals, func(i int, j int) bool {
		return terminals[i].Index > terminals[j].Index
	})
	chain := walkTranscriptParentChain(terminals[0], byUUID)
	slices.Reverse(chain)
	return includeParallelTranscriptTaskEntries(entries, chain, shouldSkip)
}

func indexTranscriptEntries(entries []transcriptEntry) (map[string]transcriptEntry, map[string]struct{}) {
	byUUID := make(map[string]transcriptEntry, len(entries))
	for _, entry := range entries {
		uuid := stringFromAny(entry.Data["uuid"])
		if uuid == "" {
			continue
		}
		current, exists := byUUID[uuid]
		if !exists || preferTranscriptEntry(current, entry) {
			byUUID[uuid] = entry
		}
	}
	parentUUIDs := make(map[string]struct{}, len(byUUID))
	for uuid, entry := range byUUID {
		if parentUUID := transcriptChainParentUUID(entry.Data); parentUUID != "" && parentUUID != uuid {
			parentUUIDs[parentUUID] = struct{}{}
		}
	}
	return byUUID, parentUUIDs
}

func preferTranscriptEntry(current transcriptEntry, candidate transcriptEntry) bool {
	uuid := stringFromAny(candidate.Data["uuid"])
	currentSelfReferences := stringFromAny(current.Data["parentUuid"]) == uuid
	candidateSelfReferences := stringFromAny(candidate.Data["parentUuid"]) == uuid
	if currentSelfReferences != candidateSelfReferences {
		return !candidateSelfReferences
	}
	// 同一 UUID 的普通重复快照仍以后写为准；只有自指副本不能覆盖有效父链。
	return candidate.Index > current.Index
}

func transcriptTerminalEntries(
	entriesByUUID map[string]transcriptEntry,
	parentUUIDs map[string]struct{},
	shouldSkip func(map[string]any) bool,
) []transcriptEntry {
	terminals := make([]transcriptEntry, 0)
	for _, entry := range entriesByUUID {
		uuid := stringFromAny(entry.Data["uuid"])
		_, isParent := parentUUIDs[uuid]
		if uuid != "" && !isParent && !shouldSkip(entry.Data) {
			terminals = append(terminals, entry)
		}
	}
	return terminals
}

func walkTranscriptParentChain(leaf transcriptEntry, byUUID map[string]transcriptEntry) []transcriptEntry {
	chain := make([]transcriptEntry, 0, len(byUUID))
	seen := make(map[string]struct{}, len(byUUID))
	current := leaf
	for {
		uuid := stringFromAny(current.Data["uuid"])
		if uuid == "" {
			break
		}
		if _, exists := seen[uuid]; exists {
			break
		}
		seen[uuid] = struct{}{}
		chain = append(chain, current)
		parentUUID := transcriptChainParentUUID(current.Data)
		if parentUUID == "" {
			break
		}
		parent, exists := byUUID[parentUUID]
		if !exists {
			break
		}
		current = parent
	}
	return chain
}

// transcriptChainParentUUID 返回条目在主链上的上一跳。
// compact_boundary 的 parentUuid 为 null，只用 logicalParentUuid 指回压缩前的链尾；
// 只认 parentUuid 会让每次自动压缩都把之前的整段历史从读模型里截掉。
func transcriptChainParentUUID(entry map[string]any) string {
	return textutil.FirstNonEmpty(
		stringFromAny(entry["parentUuid"]),
		stringFromAny(entry["logicalParentUuid"]),
	)
}

func shouldSkipTranscriptEntry(entry map[string]any) bool {
	if boolValueAny(entry["isSidechain"]) || isMetaTranscriptEntry(entry) {
		return true
	}
	if isInternalTranscriptContinuationEntry(entry) {
		return true
	}
	return stringFromAny(entry["teamName"]) != ""
}

func shouldSkipExplicitTranscriptEntry(entry map[string]any) bool {
	return isMetaTranscriptEntry(entry) ||
		isInternalTranscriptContinuationEntry(entry) ||
		stringFromAny(entry["teamName"]) != ""
}

// isMetaTranscriptEntry 识别只供模型或 transcript 内部使用的条目。
// 压缩摘要（isCompactSummary）是 runtime 注入的伪 user 输入，既不能展示，
// 也不能当作用户轮次去消费 round marker，否则后续轮次会整体错位。
func isMetaTranscriptEntry(entry map[string]any) bool {
	return boolValueAny(entry["isMeta"]) || boolValueAny(entry["is_meta"]) ||
		boolValueAny(entry["isCompactSummary"]) ||
		boolValueAny(entry["isVisibleInTranscriptOnly"])
}

func isInternalTranscriptContinuationEntry(entry map[string]any) bool {
	if stringFromAny(entry["type"]) != "user" {
		return false
	}
	return message.IsInternalTranscriptContinuationPrompt(transcriptRawUserContent(entry))
}

func includeParallelTranscriptTaskEntries(
	entries []transcriptEntry,
	chain []transcriptEntry,
	shouldSkip func(map[string]any) bool,
) []transcriptEntry {
	if len(chain) == 0 {
		return chain
	}
	chainUUIDs, toolUseIDs, seenToolResultIDs := indexTranscriptChain(chain)
	if len(toolUseIDs) == 0 {
		return chain
	}
	next := slices.Clone(chain)
	for _, entry := range entries {
		if includeParallelTranscriptEntry(entry, chainUUIDs, toolUseIDs, seenToolResultIDs, shouldSkip) ||
			includeParallelSubagentAttachment(entry, toolUseIDs, shouldSkip) {
			next = append(next, entry)
			chainUUIDs[stringFromAny(entry.Data["uuid"])] = struct{}{}
		}
	}
	sort.SliceStable(next, func(i int, j int) bool {
		return next[i].Index < next[j].Index
	})
	return next
}

func includeParallelSubagentAttachment(
	entry transcriptEntry,
	toolUseIDs map[string]struct{},
	shouldSkip func(map[string]any) bool,
) bool {
	if shouldSkip(entry.Data) || stringFromAny(entry.Data["type"]) != "attachment" {
		return false
	}
	attachment, _ := entry.Data["attachment"].(map[string]any)
	if stringFromAny(attachment["type"]) != "structured_output" {
		return false
	}
	data, _ := attachment["data"].(map[string]any)
	agentID := textutil.FirstNonEmpty(stringFromAny(data["agent_id"]), stringFromAny(data["agentId"]))
	toolUseID := textutil.FirstNonEmpty(stringFromAny(data["tool_use_id"]), stringFromAny(data["toolUseId"]))
	if agentID == "" || toolUseID == "" {
		return false
	}
	_, expected := toolUseIDs[toolUseID]
	return expected
}

func indexTranscriptChain(chain []transcriptEntry) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	chainUUIDs := make(map[string]struct{}, len(chain))
	toolUseIDs := make(map[string]struct{})
	seenToolResultIDs := make(map[string]struct{})
	for _, entry := range chain {
		if uuid := stringFromAny(entry.Data["uuid"]); uuid != "" {
			chainUUIDs[uuid] = struct{}{}
		}
		for _, toolUseID := range transcriptToolUseIDs(entry.Data) {
			toolUseIDs[toolUseID] = struct{}{}
		}
		for _, toolResultID := range transcriptToolResultIDs(entry.Data) {
			seenToolResultIDs[toolResultID] = struct{}{}
		}
	}
	return chainUUIDs, toolUseIDs, seenToolResultIDs
}

func includeParallelTranscriptEntry(
	entry transcriptEntry,
	chainUUIDs map[string]struct{},
	toolUseIDs map[string]struct{},
	seenToolResultIDs map[string]struct{},
	shouldSkip func(map[string]any) bool,
) bool {
	uuid := stringFromAny(entry.Data["uuid"])
	if uuid == "" || shouldSkip(entry.Data) {
		return false
	}
	if _, exists := chainUUIDs[uuid]; exists {
		return false
	}
	if _, parentInChain := chainUUIDs[stringFromAny(entry.Data["parentUuid"])]; !parentInChain {
		return false
	}
	matched := false
	for _, toolResultID := range transcriptToolResultIDs(entry.Data) {
		_, expected := toolUseIDs[toolResultID]
		_, seen := seenToolResultIDs[toolResultID]
		if expected && !seen {
			seenToolResultIDs[toolResultID] = struct{}{}
			matched = true
		}
	}
	return matched
}

func transcriptToolUseIDs(entry map[string]any) []string {
	ids := make([]string, 0)
	for _, block := range transcriptContentBlocks(entry) {
		if stringFromAny(block["type"]) != "tool_use" {
			continue
		}
		if toolUseID := stringFromAny(block["id"]); toolUseID != "" {
			ids = append(ids, toolUseID)
		}
	}
	return ids
}

func transcriptToolResultIDs(entry map[string]any) []string {
	ids := make([]string, 0)
	for _, block := range transcriptContentBlocks(entry) {
		if stringFromAny(block["type"]) != "tool_result" {
			continue
		}
		if toolUseID := stringFromAny(block["tool_use_id"]); toolUseID != "" {
			ids = append(ids, toolUseID)
		}
	}
	return ids
}

func transcriptContentBlocks(entry map[string]any) []map[string]any {
	messageValue, ok := entry["message"].(map[string]any)
	if !ok {
		return nil
	}
	switch contentValue := messageValue["content"].(type) {
	case []any:
		blocks := make([]map[string]any, 0, len(contentValue))
		for _, item := range contentValue {
			block, ok := item.(map[string]any)
			if ok {
				blocks = append(blocks, block)
			}
		}
		return blocks
	case []map[string]any:
		return contentValue
	default:
		return nil
	}
}
