package feishudocx

import (
	"encoding/json"
	"strings"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

func sdkBlocksToMaps(blocks []*larkdocx.Block) ([]Block, error) {
	if len(blocks) == 0 {
		return []Block{}, nil
	}
	result := make([]Block, 0, len(blocks))
	for _, block := range blocks {
		item, err := sdkBlockToMap(block)
		if err != nil {
			return nil, err
		}
		if item != nil {
			result = append(result, item)
		}
	}
	return result, nil
}

func sdkBlockToMap(block *larkdocx.Block) (Block, error) {
	if block == nil {
		return nil, nil
	}
	payload, err := json.Marshal(block)
	if err != nil {
		return nil, err
	}
	var result Block
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func mapsToSDKBlocks(blocks []Block) ([]*larkdocx.Block, error) {
	if len(blocks) == 0 {
		return []*larkdocx.Block{}, nil
	}
	payload, err := json.Marshal(blocks)
	if err != nil {
		return nil, err
	}
	var result []*larkdocx.Block
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func sdkObjectsToMaps(value any) ([]map[string]any, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if string(payload) == "null" {
		return []map[string]any{}, nil
	}
	var result []map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return []map[string]any{}, nil
	}
	return result, nil
}

func sdkObjectToMap(value any) (map[string]any, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if string(payload) == "null" {
		return map[string]any{}, nil
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func filterDescendants(blocks []Block, firstLevelIDs []string) []Block {
	byID := map[string]Block{}
	for _, block := range blocks {
		if id := blockID(block); id != "" {
			byID[id] = block
		}
	}
	seen := map[string]bool{}
	var result []Block
	var walk func(string)
	walk = func(id string) {
		if seen[id] {
			return
		}
		block, ok := byID[id]
		if !ok {
			return
		}
		seen[id] = true
		result = append(result, block)
		for _, childID := range blockChildren(block) {
			walk(childID)
		}
	}
	for _, id := range firstLevelIDs {
		walk(id)
	}
	return result
}

func chunkStrings(values []string, size int) [][]string {
	if size <= 0 || len(values) == 0 {
		return nil
	}
	var result [][]string
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		result = append(result, values[start:end])
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
