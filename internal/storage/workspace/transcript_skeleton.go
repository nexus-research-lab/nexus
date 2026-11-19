// INPUT: 已被 DM 投影检查点覆盖的 transcript 解析条目。
// OUTPUT: 只保留建链、并行工具关联、跳过判定与 round marker 对齐所需字段的骨架条目。
// POS: 大 transcript 解析缓存的瘦身边界；骨架条目绝不交给 Processor 投影。
package workspace

import "slices"

// transcriptSkeletonKeys 是建链（uuid/父链）、跳过判定（sidechain/meta/team）与
// 时间推断读取的顶层字段；message 与 attachment 另行收缩。
var transcriptSkeletonKeys = []string{
	"uuid", "parentUuid", "logicalParentUuid", "type", "timestamp", "teamName",
	"isSidechain", "isMeta", "is_meta", "isCompactSummary", "isVisibleInTranscriptOnly",
}

// slimTranscriptEntries 返回把 Index 落在 [from, to) 的条目替换为骨架后的新切片。
// 原切片可能仍被其他读取方持有，因此只复制切片头、不就地修改。
func slimTranscriptEntries(entries []transcriptEntry, from int, to int) ([]transcriptEntry, int) {
	if to <= from {
		return entries, from
	}
	slimmed := slices.Clone(entries)
	for i, entry := range slimmed {
		if entry.Index >= to {
			break
		}
		if entry.Index >= from {
			slimmed[i].Data = skeletonTranscriptEntry(entry.Data)
		}
	}
	return slimmed, to
}

// skeletonTranscriptEntry 收缩单个条目。真正的用户输入（不含 tool_result 的 user 条目）
// 原样保留：marker 对齐要解码其内容、比对文本与 Goal/自唤醒形态，且这类条目本身很小。
// 其余条目的体量集中在回复文本、思考与工具结果上，与上述判定无关，可以丢弃。
func skeletonTranscriptEntry(data map[string]any) map[string]any {
	isUser := stringFromAny(data["type"]) == "user"
	if isUser && len(transcriptToolResultIDs(data)) == 0 {
		return data
	}
	skeleton := make(map[string]any, len(transcriptSkeletonKeys)+2)
	for _, key := range transcriptSkeletonKeys {
		if value, ok := data[key]; ok {
			skeleton[key] = value
		}
	}
	if blocks := skeletonTranscriptContent(transcriptContentBlocks(data), isUser); blocks != nil {
		skeleton["message"] = map[string]any{"content": blocks}
	}
	if attachment := skeletonTranscriptAttachment(data["attachment"]); attachment != nil {
		skeleton["attachment"] = attachment
	}
	return skeleton
}

// skeletonTranscriptContent 只留工具调用与结果的配对 id；user 条目的文本块保留，
// 内部续跑提示的识别依赖它们。
func skeletonTranscriptContent(blocks []map[string]any, isUser bool) []any {
	if len(blocks) == 0 {
		return nil
	}
	kept := make([]any, 0, len(blocks))
	for _, block := range blocks {
		switch blockType := stringFromAny(block["type"]); {
		case blockType == "tool_use":
			kept = append(kept, map[string]any{"type": blockType, "id": block["id"]})
		case blockType == "tool_result":
			kept = append(kept, map[string]any{"type": blockType, "tool_use_id": block["tool_use_id"]})
		case blockType == "text" && isUser:
			kept = append(kept, map[string]any{"type": blockType, "text": block["text"]})
		}
	}
	return kept
}

// skeletonTranscriptAttachment 只保留并行子任务结构化输出的关联 id。
func skeletonTranscriptAttachment(value any) map[string]any {
	attachment, _ := value.(map[string]any)
	if stringFromAny(attachment["type"]) != "structured_output" {
		return nil
	}
	data, _ := attachment["data"].(map[string]any)
	ids := make(map[string]any, 2)
	for _, key := range []string{"agent_id", "agentId", "tool_use_id", "toolUseId"} {
		if item, ok := data[key]; ok {
			ids[key] = item
		}
	}
	return map[string]any{"type": "structured_output", "data": ids}
}
