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

	byUUID := make(map[string]transcriptEntry, len(entries))
	parentUUIDs := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		uuid := stringFromAny(entry.Data["uuid"])
		if uuid == "" {
			continue
		}
		byUUID[uuid] = entry
		parentUUID := stringFromAny(entry.Data["parentUuid"])
		if parentUUID != "" {
			parentUUIDs[parentUUID] = struct{}{}
		}
	}

	terminals := make([]transcriptEntry, 0)
	for _, entry := range entries {
		uuid := stringFromAny(entry.Data["uuid"])
		if uuid == "" {
			continue
		}
		if _, exists := parentUUIDs[uuid]; exists {
			continue
		}
		if shouldSkip(entry.Data) {
			continue
		}
		terminals = append(terminals, entry)
	}
	if len(terminals) == 0 {
		return nil
	}

	sort.Slice(terminals, func(i int, j int) bool {
		return terminals[i].Index > terminals[j].Index
	})

	leaf := terminals[0]
	chain := make([]transcriptEntry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
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

	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}
	return includeParallelTranscriptToolResults(entries, chain, shouldSkip)
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
	if len(toolUseIDs) == 0 {
		return chain
	}

	next := slices.Clone(chain)
	for _, entry := range entries {
		uuid := stringFromAny(entry.Data["uuid"])
		if uuid == "" {
			continue
		}
		if _, exists := chainUUIDs[uuid]; exists {
			continue
		}
		if shouldSkip(entry.Data) {
			continue
		}
		parentUUID := stringFromAny(entry.Data["parentUuid"])
		if _, parentInChain := chainUUIDs[parentUUID]; !parentInChain {
			continue
		}

		matched := false
		for _, toolResultID := range transcriptToolResultIDs(entry.Data) {
			if _, exists := toolUseIDs[toolResultID]; !exists {
				continue
			}
			if _, exists := seenToolResultIDs[toolResultID]; exists {
				continue
			}
			seenToolResultIDs[toolResultID] = struct{}{}
			matched = true
		}
		if !matched {
			continue
		}
		next = append(next, entry)
		chainUUIDs[uuid] = struct{}{}
	}

	sort.SliceStable(next, func(i int, j int) bool {
		return next[i].Index < next[j].Index
	})
	return next
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
