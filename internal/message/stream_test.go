// stream_test.go 覆盖流式消息、文件投递与 WorkGraph/Workspace 产物投影测试。
package message

import (
	"encoding/json"
	"strconv"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProcessorAlignsAssistantSequenceWithPythonSemantics(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-processor",
		ParentID:   "round-processor",
	}, "")

	startOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-processor",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-processor-1",
					"model": "sonnet",
				},
			},
		},
	})
	if !startOutput.StreamStarted || len(startOutput.StreamEvents) != 1 {
		t.Fatalf("message_start 未建立流式段: %+v", startOutput)
	}

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "先分析",
				},
			},
		},
	})
	deltaOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": " 再收口",
				},
			},
		},
	})
	if len(deltaOutput.StreamEvents) != 1 {
		t.Fatalf("thinking delta 未输出 stream 事件: %+v", deltaOutput)
	}
	contentBlock, _ := deltaOutput.StreamEvents[0].Data["content_block"].(map[string]any)
	if contentBlock["thinking"] != "先分析 再收口" {
		t.Fatalf("thinking 增量被破坏: %+v", contentBlock)
	}

	taskProgressOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{
			Subtype: "task_progress",
			TaskProgress: &sdkprotocol.TaskProgressMessage{
				TaskID:       "task-1",
				LastToolName: "SearchWeb",
				Summary:      "正在整理检索结果",
			},
		},
	})
	if len(taskProgressOutput.DurableMessages) != 1 {
		t.Fatalf("task_progress 未并入 assistant durable 消息: %+v", taskProgressOutput)
	}
	progressBlocks, _ := taskProgressOutput.DurableMessages[0]["content"].([]map[string]any)
	if len(progressBlocks) != 2 || progressBlocks[1]["type"] != "task_progress" {
		t.Fatalf("task_progress 内容块不正确: %+v", taskProgressOutput.DurableMessages[0])
	}

	terminalOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "end_turn",
				},
			},
		},
	})
	if len(terminalOutput.DurableMessages) != 1 || !terminalOutput.AssistantCompleted {
		t.Fatalf("message_delta 未补出 durable assistant 快照: %+v", terminalOutput)
	}
	assistantMessage := terminalOutput.DurableMessages[0]
	if assistantMessage["role"] != "assistant" || assistantMessage["stop_reason"] != "end_turn" {
		t.Fatalf("assistant 快照不正确: %+v", assistantMessage)
	}
	assistantBlocks, _ := assistantMessage["content"].([]map[string]any)
	if len(assistantBlocks) != 2 || assistantBlocks[0]["type"] != "thinking" || assistantBlocks[1]["type"] != "task_progress" {
		t.Fatalf("assistant 快照内容顺序不正确: %+v", assistantBlocks)
	}

	resultOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-processor-1",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 12,
			NumTurns:   1,
			Result:     "done",
		},
	})
	if resultOutput.TerminalStatus != "finished" || resultOutput.ResultSubtype != "success" {
		t.Fatalf("result 终态不正确: %+v", resultOutput)
	}
	if len(resultOutput.DurableMessages) != 1 || resultOutput.DurableMessages[0]["role"] != "result" {
		t.Fatalf("result durable 消息不正确: %+v", resultOutput.DurableMessages)
	}
}

func TestProcessorStreamsVisualizeWidgetCodeBeforeInputJSONCompletes(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-widget-stream"}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-widget-stream"},
		}},
	})
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    "widget-1",
				"name":  "mcp__nexus__show_widget",
				"input": map[string]any{},
			},
		}},
	})

	partial := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": `{"title":"增长曲线","widget_code":"<div class=\"chart\">\u4e`,
			},
		}},
	})
	block, _ := partial.StreamEvents[0].Data["content_block"].(map[string]any)
	input, _ := block["input"].(map[string]any)
	if input["title"] != "增长曲线" || input["widget_code"] != `<div class="chart">` {
		t.Fatalf("partial widget input = %+v", input)
	}

	complete := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": `2d</div>"}`,
			},
		}},
	})
	block, _ = complete.StreamEvents[0].Data["content_block"].(map[string]any)
	input, _ = block["input"].(map[string]any)
	if input["title"] != "增长曲线" || input["widget_code"] != `<div class="chart">中</div>` {
		t.Fatalf("complete widget input = %+v", input)
	}
}

func TestProcessorRecoversInputJSONDeltaWithoutBlockStart(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-input-delta"}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-input-delta"},
		}},
	})

	delta := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": `{"command":"pwd"}`,
			},
		}},
	})
	if len(delta.StreamEvents) != 1 {
		t.Fatalf("orphan input_json_delta should remain observable: %+v", delta)
	}
	block, _ := delta.StreamEvents[0].Data["content_block"].(map[string]any)
	input, _ := block["input"].(map[string]any)
	if block["type"] != "tool_use" || input["command"] != "pwd" {
		t.Fatalf("orphan input_json_delta recovery = %+v", block)
	}

	final := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-input-delta",
			StopReason: "tool_use",
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
				ID:    "tool-input-delta",
				Name:  "Bash",
				Input: []byte(`{"command":"pwd"}`),
			}},
		}},
	})
	if len(final.DurableMessages) != 1 {
		t.Fatalf("final tool snapshot = %+v, want one durable message", final)
	}
	blocks, _ := final.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["id"] != "tool-input-delta" {
		t.Fatalf("stream placeholder was not replaced by final tool snapshot: %+v", blocks)
	}
}

func TestProcessorDefersAssistantCompletionUntilStreamTerminal(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-live-terminal",
		ParentID:   "round-live-terminal",
	}, "sdk-session-live-terminal")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-live-terminal-1",
					"model": "glm-5-turbo",
				},
			},
		},
	})
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type":     "thinking",
					"thinking": "先分析",
				},
			},
		},
	})

	thinkingSnapshot := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-live-terminal-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
				},
			},
		},
	})
	if len(thinkingSnapshot.DurableMessages) != 1 {
		t.Fatalf("thinking 快照应落一条中间 durable assistant: %+v", thinkingSnapshot)
	}
	if thinkingSnapshot.DurableMessages[0]["is_complete"] != false {
		t.Fatalf("流式中的 thinking 快照不应提前完成: %+v", thinkingSnapshot.DurableMessages[0])
	}
	if _, ok := thinkingSnapshot.DurableMessages[0]["stop_reason"]; ok {
		t.Fatalf("流式中的 thinking 快照不应暴露 stop_reason: %+v", thinkingSnapshot.DurableMessages[0])
	}

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type": "text",
					"text": "最终回答",
				},
			},
		},
	})
	textSnapshot := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-live-terminal-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
					sdkprotocol.TextBlock{Text: "最终回答"},
				},
			},
		},
	})
	if len(textSnapshot.DurableMessages) != 1 {
		t.Fatalf("文本快照应继续落中间 durable assistant: %+v", textSnapshot)
	}
	if textSnapshot.DurableMessages[0]["is_complete"] != false {
		t.Fatalf("message_delta 之前不应把 assistant 标记完成: %+v", textSnapshot.DurableMessages[0])
	}
	if _, ok := textSnapshot.DurableMessages[0]["stop_reason"]; ok {
		t.Fatalf("message_delta 之前的文本快照不应暴露 stop_reason: %+v", textSnapshot.DurableMessages[0])
	}

	terminalOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "end_turn",
				},
			},
		},
	})
	if len(terminalOutput.DurableMessages) != 1 || !terminalOutput.AssistantCompleted {
		t.Fatalf("message_delta 应补出唯一终态 assistant: %+v", terminalOutput)
	}
	if terminalOutput.DurableMessages[0]["is_complete"] != true {
		t.Fatalf("终态 assistant 应标记完成: %+v", terminalOutput.DurableMessages[0])
	}
	if terminalOutput.DurableMessages[0]["stop_reason"] != "end_turn" {
		t.Fatalf("终态 assistant 应携带 stop_reason: %+v", terminalOutput.DurableMessages[0])
	}

	duplicateSnapshot := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-live-terminal-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先分析"},
					sdkprotocol.TextBlock{Text: "最终回答"},
				},
			},
		},
	})
	if len(duplicateSnapshot.DurableMessages) != 0 {
		t.Fatalf("终态 assistant 重复快照不应重复落库: %+v", duplicateSnapshot)
	}
}

func TestFileDeliveryPersistsExactAgentRound(t *testing.T) {
	for _, agent := range []string{"researcher", "designer"} {
		t.Run(agent, func(t *testing.T) {
			p := NewProcessor(MessageContext{AgentID: agent, AgentRoundID: agent + "-round", RoomID: "room", ConversationID: "conversation", RoundID: "root", WorkspacePath: t.TempDir()}, "")
			p.Process(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant, Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{ID: "reply-" + agent, Content: []sdkprotocol.ContentBlock{
				sdkprotocol.ToolUseBlock{ID: "delivery", Name: "mcp__nexus__deliver_files", Input: json.RawMessage(`{"paths":["report.pptx","data.xlsx"]}`)},
			}}}})
			text, _ := json.Marshal(map[string]any{"kind": "file_delivery", "version": 1, "agent_id": agent, "agent_round_id": agent + "-round", "paths": []string{"report.pptx", "data.xlsx"}})
			content, _ := json.Marshal(string(text))
			out := p.Process(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeUser, User: &sdkprotocol.UserMessage{Message: sdkprotocol.ConversationEnvelope{Content: []sdkprotocol.ContentBlock{
				sdkprotocol.ToolResultBlock{ToolUseID: "delivery", Content: json.RawMessage(content)},
			}}}})
			if len(out.DurableMessages) != 1 {
				t.Fatalf("missing durable delivery: %+v", out)
			}
			blocks := out.DurableMessages[0]["content"].([]map[string]any)
			if len(blocks) != 4 {
				t.Fatalf("missing output files: %+v", blocks)
			}
			for _, block := range blocks[2:] {
				if block["workspace_agent_id"] != agent || block["producer_agent_id"] != agent || block["source_agent_round_id"] != agent+"-round" || block["role"] != "deliverable" {
					t.Fatalf("wrong provenance: %+v", block)
				}
			}
		})
	}
}

func TestFileDeliveryRejectsUntrustedOrInvalidEvidence(t *testing.T) {
	for _, test := range []struct {
		name, tool, agent, round, path string
		failed                         bool
	}{
		{"shell text", "Bash", "a", "r", "out.pdf", false},
		{"other MCP", "mcp__other__deliver_files", "a", "r", "out.pdf", false},
		{"other agent", "nexus.deliver_files", "b", "r", "out.pdf", false},
		{"old round", "nexus.deliver_files", "a", "old", "out.pdf", false},
		{"outside", "nexus.deliver_files", "a", "r", "../out.pdf", false},
		{"empty", "nexus.deliver_files", "a", "r", "", false},
		{"failed", "nexus.deliver_files", "a", "r", "out.pdf", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := NewProcessor(MessageContext{AgentID: "a", AgentRoundID: "r", WorkspacePath: t.TempDir()}, "")
			p.segment.AppendToolResults([]map[string]any{{"type": "tool_use", "id": "tool", "name": test.tool}})
			data, _ := json.Marshal(map[string]any{"kind": "file_delivery", "version": 1, "agent_id": test.agent, "agent_round_id": test.round, "paths": []string{"valid.pdf", test.path}})
			if got := p.workspaceFileArtifactsForToolResult(map[string]any{"tool_use_id": "tool", "content": string(data), "is_error": test.failed}); len(got) != 0 {
				t.Fatalf("accepted invalid evidence: %+v", got)
			}
		})
	}
}

func TestProcessorAddsWorkGraphArtifactForManagedAuthoringResult(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test", AgentID: "nexus",
		RoundID: "round-workgraph-artifact", ParentID: "round-workgraph-artifact",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
				ID: "tool-workgraph-1", Name: "Bash",
				Input: json.RawMessage(`{"command":"\"${NEXUS_COMMAND_PATH}\" --json execution invoke --operation 'revise_workgraph_preview' --request-id 'revise-1234'"}`),
			}},
		}},
	})
	payload := `{"domain":"execution","action":"invoke","operation":"revise_workgraph_preview","is_error":false,"data":{"draft":{"preview_id":"preview-1","head_revision":2,"selected_revision":2,"versions":[{"revision":1},{"revision":2}],"preview":{"preview_id":"preview-1","slash_name":"briefing","title":"协作简报","source_execution_id":"execution-1","source_session_key":"agent:nexus:ws:dm:test","objective":"形成简报","nodes":[{"logical_key":"draft","role":"key","kind":"produce","subject":"起草","objective":"起草简报","deliverable":"简报","required":true,"terminal":true,"position":0}],"expires_at":"2026-08-22T00:00:00Z"}}}}`
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolResultBlock{
				ToolUseID: "tool-workgraph-1", Content: json.RawMessage(strconv.Quote(payload)),
			}},
		}},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("workgraph artifact durable output = %#v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 {
		t.Fatalf("workgraph artifact blocks = %#v", blocks)
	}
	artifact := blocks[2]
	if artifact["type"] != protocol.ContentBlockTypeWorkGraphArtifact ||
		artifact["state"] != protocol.WorkGraphArtifactStateDraft ||
		artifact["head_revision"] != int64(2) || artifact["version_count"] != 2 {
		t.Fatalf("workgraph artifact metadata = %#v", artifact)
	}
	preview, ok := artifact["preview"].(*protocol.WorkGraphWorkflowPreview)
	if !ok || preview.PreviewID != "preview-1" || len(preview.Nodes) != 1 {
		t.Fatalf("workgraph artifact preview = %#v", artifact["preview"])
	}
}

func TestProcessorAddsWorkGraphArtifactForStructuredRuntimeCommand(t *testing.T) {
	structured := map[string]any{
		"preview_id": "preview-1", "head_revision": float64(1), "selected_revision": float64(1),
		"versions": []any{map[string]any{"revision": float64(1)}},
		"preview": map[string]any{
			"preview_id": "preview-1", "slash_name": "briefing", "title": "协作简报",
			"source_execution_id": "execution-1", "source_session_key": "agent:nexus:ws:dm:test",
			"objective": "形成简报", "expires_at": "2026-08-22T00:00:00Z",
			"nodes": []any{map[string]any{
				"logical_key": "draft", "role": "key", "kind": "produce", "subject": "起草",
				"objective": "起草简报", "deliverable": "简报", "required": true, "terminal": true,
				"position": float64(0),
			}},
		},
	}
	for _, testCase := range []struct {
		name          string
		toolUseResult map[string]any
	}{
		{name: "flat compatibility", toolUseResult: structured},
		{name: "wrapped MCP result", toolUseResult: map[string]any{
			"content": structured, "structuredContent": structured,
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			processor := NewProcessor(MessageContext{
				SessionKey: "agent:nexus:ws:dm:test", AgentID: "nexus",
				RoundID: "round-workgraph-native", ParentID: "round-workgraph-native",
			}, "")
			processor.Process(sdkprotocol.ReceivedMessage{
				Type: sdkprotocol.MessageTypeAssistant,
				Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
					Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
						ID: "tool-workgraph-native", Name: "mcp__nexus__command",
						Input: json.RawMessage(`{"domain":"execution","action":"invoke","operation":"get_workgraph_preview","request_id":"get-preview-native-1","input":{"preview_id":"preview-1"}}`),
					}},
				}},
			})
			output := processor.Process(sdkprotocol.ReceivedMessage{
				Type: sdkprotocol.MessageTypeUser,
				User: &sdkprotocol.UserMessage{
					ToolUseResult: testCase.toolUseResult,
					Message: sdkprotocol.ConversationEnvelope{Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-workgraph-native", Content: json.RawMessage(`"updated"`),
					}}},
				},
			})
			blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
			if len(blocks) != 3 || blocks[2]["type"] != protocol.ContentBlockTypeWorkGraphArtifact {
				t.Fatalf("structured runtime command artifact blocks = %#v", blocks)
			}
		})
	}
}

func TestProcessorAddsImagegenArtifactFromBashResult(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey:    "agent:nexus:ws:dm:test",
		AgentID:       "nexus",
		WorkspacePath: t.TempDir(),
		RoundID:       "round-imagegen-artifact",
		ParentID:      "round-imagegen-artifact",
	}, "")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-imagegen-artifact-1",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-bash-imagegen-1",
						Name:  "Bash",
						Input: json.RawMessage(`{"command":"nexusctl imagegen generate --prompt fox --file-name fox"}`),
					},
				},
			},
		},
	})

	imagegenOutput := `{"domain":"imagegen","action":"generate","item":{"provider":"azure","model":"gpt-image-2","path":"output/imagegen/fox.png","mime_type":"image/png","markdown":"![generated image](output/imagegen/fox.png)"},"payload_bytes":123}`
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-bash-imagegen-1",
						Content:   json.RawMessage(strconv.Quote(imagegenOutput)),
						IsError:   false,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("imagegen artifact 未生成 durable assistant 消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 {
		t.Fatalf("imagegen artifact 内容块数量不正确: %+v", blocks)
	}
	artifact := blocks[2]
	if artifact["type"] != protocol.ContentBlockTypeWorkspaceFileArtifact {
		t.Fatalf("第三块应为 workspace_file_artifact: %+v", artifact)
	}
	if artifact["path"] != "output/imagegen/fox.png" || artifact["artifact_kind"] != protocol.WorkspaceFileArtifactKindImage {
		t.Fatalf("imagegen artifact 路径或类型不正确: %+v", artifact)
	}
	if artifact["mime_type"] != "image/png" || artifact["label"] != "生成图片" {
		t.Fatalf("imagegen artifact 元数据不正确: %+v", artifact)
	}
	if artifact["source_tool_use_id"] != "tool-bash-imagegen-1" || artifact["source_tool_name"] != "Bash" {
		t.Fatalf("imagegen artifact 来源工具不正确: %+v", artifact)
	}
}
