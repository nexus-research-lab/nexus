package workspace

import "testing"

func TestPrimaryTranscriptChainPrefersNonSelfReferentialDuplicateUUID(t *testing.T) {
	entries := []transcriptEntry{
		{
			Index: 0,
			Data: map[string]any{
				"type": "user",
				"uuid": "user-before-memory",
				"message": map[string]any{
					"role":    "user",
					"content": "保留这条输入",
				},
			},
		},
		{
			Index: 1,
			Data: map[string]any{
				"type":       "system",
				"uuid":       "runtime-memory-saved",
				"parentUuid": "user-before-memory",
				"subtype":    "memory_saved",
			},
		},
		{
			Index: 2,
			Data: map[string]any{
				"type":       "system",
				"uuid":       "runtime-memory-saved",
				"parentUuid": "runtime-memory-saved",
				"subtype":    "memory_saved",
			},
		},
		{
			Index: 3,
			Data: map[string]any{
				"type":       "assistant",
				"uuid":       "assistant-after-memory",
				"parentUuid": "runtime-memory-saved",
				"message": map[string]any{
					"role":    "assistant",
					"content": []any{map[string]any{"type": "text", "text": "历史仍连续"}},
				},
			},
		},
	}

	chain := buildPrimaryTranscriptChain(entries)
	if len(chain) != 3 {
		t.Fatalf("重复 memory UUID 不应截断主链: %+v", chain)
	}
	if got := stringFromAny(chain[0].Data["uuid"]); got != "user-before-memory" {
		t.Fatalf("主链丢失 memory 事件之前的用户输入: got=%q chain=%+v", got, chain)
	}
	if got := stringFromAny(chain[1].Data["parentUuid"]); got != "user-before-memory" {
		t.Fatalf("应保留非自指的 memory 事件副本: got=%q chain=%+v", got, chain)
	}
}

func TestPrimaryTranscriptChainIgnoresSelfReferentialDuplicateTerminal(t *testing.T) {
	entries := []transcriptEntry{
		{
			Index: 0,
			Data: map[string]any{
				"type": "user",
				"uuid": "user-before-memory",
				"message": map[string]any{
					"role":    "user",
					"content": "终点也必须保留",
				},
			},
		},
		{
			Index: 1,
			Data: map[string]any{
				"type":       "system",
				"uuid":       "runtime-memory-saved",
				"parentUuid": "user-before-memory",
				"subtype":    "memory_saved",
			},
		},
		{
			Index: 2,
			Data: map[string]any{
				"type":       "system",
				"uuid":       "runtime-memory-saved",
				"parentUuid": "runtime-memory-saved",
				"subtype":    "memory_saved",
			},
		},
	}

	chain := buildPrimaryTranscriptChain(entries)
	if len(chain) != 2 {
		t.Fatalf("terminal 自指副本不应覆盖有效主链: %+v", chain)
	}
	if got := stringFromAny(chain[0].Data["uuid"]); got != "user-before-memory" {
		t.Fatalf("terminal 自指副本导致用户输入丢失: got=%q chain=%+v", got, chain)
	}
	if got := stringFromAny(chain[1].Data["parentUuid"]); got != "user-before-memory" {
		t.Fatalf("terminal 应使用非自指副本: got=%q chain=%+v", got, chain)
	}
}

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
