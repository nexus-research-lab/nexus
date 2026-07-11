package workspace

import (
	"bufio"
	"encoding/json"
	"os"
	"slices"
	"sort"
	"strings"
)

func (s *AgentHistoryStore) readTranscriptEntries(path string) ([]transcriptEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewScanner(file)
	reader.Buffer(make([]byte, 0, transcriptReadBufferBytes), transcriptScannerBufferBytes)

	results := make([]transcriptEntry, 0)
	for index := 0; reader.Scan(); index++ {
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}
		entry := map[string]any{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
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
	return results, reader.Err()
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
	terminals := transcriptTerminalEntries(entries, parentUUIDs, shouldSkip)
	if len(terminals) == 0 {
		return nil
	}
	sort.Slice(terminals, func(i int, j int) bool {
		return terminals[i].Index > terminals[j].Index
	})
	chain := walkTranscriptParentChain(terminals[0], byUUID)
	slices.Reverse(chain)
	return includeParallelTranscriptToolResults(entries, chain, shouldSkip)
}

func indexTranscriptEntries(entries []transcriptEntry) (map[string]transcriptEntry, map[string]struct{}) {
	byUUID := make(map[string]transcriptEntry, len(entries))
	parentUUIDs := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		uuid := stringFromAny(entry.Data["uuid"])
		if uuid == "" {
			continue
		}
		byUUID[uuid] = entry
		if parentUUID := stringFromAny(entry.Data["parentUuid"]); parentUUID != "" {
			parentUUIDs[parentUUID] = struct{}{}
		}
	}
	return byUUID, parentUUIDs
}

func transcriptTerminalEntries(
	entries []transcriptEntry,
	parentUUIDs map[string]struct{},
	shouldSkip func(map[string]any) bool,
) []transcriptEntry {
	terminals := make([]transcriptEntry, 0)
	for _, entry := range entries {
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
		parentUUID := stringFromAny(current.Data["parentUuid"])
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

func shouldSkipTranscriptEntry(entry map[string]any) bool {
	if boolValueAny(entry["isSidechain"]) || boolValueAny(entry["isMeta"]) {
		return true
	}
	return stringFromAny(entry["teamName"]) != ""
}

func shouldSkipExplicitTranscriptEntry(entry map[string]any) bool {
	return boolValueAny(entry["isMeta"]) || stringFromAny(entry["teamName"]) != ""
}

func includeParallelTranscriptToolResults(
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
		if includeParallelTranscriptEntry(entry, chainUUIDs, toolUseIDs, seenToolResultIDs, shouldSkip) {
			next = append(next, entry)
			chainUUIDs[stringFromAny(entry.Data["uuid"])] = struct{}{}
		}
	}
	sort.SliceStable(next, func(i int, j int) bool {
		return next[i].Index < next[j].Index
	})
	return next
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
