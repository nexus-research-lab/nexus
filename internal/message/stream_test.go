// stream_test.go 覆盖流式消息、文件投递与 WorkGraph/Workspace 产物投影测试。
package message

import (
	"encoding/json"
	"strconv"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
