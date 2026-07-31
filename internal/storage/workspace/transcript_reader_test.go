package workspace

import "testing"

func TestTranscriptContinuationPromptIsSkippedFromBothChains(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": "Output token limit hit. Resume directly — no apology, no recap of what you were doing. Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces.",
		},
	}

	if !shouldSkipTranscriptEntry(entry) {
		t.Fatalf("主 transcript chain 应跳过内部续跑提示")
	}
	if !shouldSkipExplicitTranscriptEntry(entry) {
		t.Fatalf("显式 transcript chain 应跳过内部续跑提示")
	}
}

func TestTranscriptContinuationPromptDoesNotSkipSimilarUserContent(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": "Output token limit hit. Resume directly — please explain what happened.",
		},
	}

	if shouldSkipTranscriptEntry(entry) || shouldSkipExplicitTranscriptEntry(entry) {
		t.Fatalf("普通用户内容不应被当成内部续跑提示")
	}
}

func TestTranscriptMetaUserIsSkippedForBothWireCasings(t *testing.T) {
	for _, key := range []string{"isMeta", "is_meta"} {
		entry := map[string]any{
			"type": "user",
			key:    true,
			"message": map[string]any{
				"role":    "user",
				"content": "完整 Skill 正文",
			},
		}
		if !shouldSkipTranscriptEntry(entry) ||
			!shouldSkipExplicitTranscriptEntry(entry) {
			t.Fatalf("%s meta user 不应进入可见 transcript", key)
		}
	}
}

func TestPrimaryTranscriptChainIncludesParallelSubagentStructuredOutput(t *testing.T) {
	entries := []transcriptEntry{
		{
			Index: 0,
			Data: map[string]any{
				"type": "user",
				"uuid": "user-1",
				"message": map[string]any{
					"role":    "user",
					"content": "拉一个子智能体",
				},
			},
		},
		{
			Index: 1,
			Data: map[string]any{
				"type":       "assistant",
				"uuid":       "assistant-agent",
				"parentUuid": "user-1",
				"message": map[string]any{
					"role": "assistant",
					"content": []any{
						map[string]any{
							"type": "tool_use",
							"id":   "call-agent",
							"name": "Agent",
						},
					},
				},
			},
		},
		{
			Index: 2,
			Data: map[string]any{
				"type":       "attachment",
				"uuid":       "agent-output",
				"parentUuid": "assistant-agent",
				"attachment": map[string]any{
					"type": "structured_output",
					"data": map[string]any{
						"agentId":   "agent-child-1",
						"toolUseId": "call-agent",
					},
				},
			},
		},
		{
			Index: 3,
			Data: map[string]any{
				"type":       "user",
				"uuid":       "tool-result",
				"parentUuid": "assistant-agent",
				"message": map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{
							"type":        "tool_result",
							"tool_use_id": "call-agent",
						},
					},
				},
			},
		},
		{
			Index: 4,
			Data: map[string]any{
				"type":       "assistant",
				"uuid":       "assistant-final",
				"parentUuid": "tool-result",
				"message": map[string]any{
					"role":    "assistant",
					"content": []any{map[string]any{"type": "text", "text": "完成"}},
				},
			},
		},
	}

	chain := buildPrimaryTranscriptChain(entries)
	found := false
	for _, entry := range chain {
		if entry.Data["uuid"] == "agent-output" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("主链遗漏与 Agent tool_use 关联的 structured output: %+v", chain)
	}

	rows := projectTranscriptChain("", "agent:host:ws:dm:test", "host", chain, nil)
	var taskNotification map[string]any
	for _, row := range rows {
		metadata, _ := row["metadata"].(map[string]any)
		if metadata["subtype"] == "task_notification" {
			taskNotification = metadata
			break
		}
	}
	if taskNotification == nil ||
		taskNotification["task_id"] != "agent-child-1" ||
		taskNotification["tool_use_id"] != "call-agent" ||
		taskNotification["task_type"] != "local_agent" {
		t.Fatalf("structured output 未投影成 subagent task notification: %+v", rows)
	}
}
