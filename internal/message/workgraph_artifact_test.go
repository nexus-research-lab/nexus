package message

import (
	"encoding/json"
	"strconv"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test", AgentID: "nexus",
		RoundID: "round-workgraph-native", ParentID: "round-workgraph-native",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
				ID: "tool-workgraph-native", Name: "mcp__nexus_runtime__command",
				Input: json.RawMessage(`{"domain":"execution","action":"invoke","operation":"get_workgraph_preview","request_id":"get-preview-native-1","input":{"preview_id":"preview-1"}}`),
			}},
		}},
	})
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
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			ToolUseResult: structured,
			Message: sdkprotocol.ConversationEnvelope{Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolResultBlock{
				ToolUseID: "tool-workgraph-native", Content: json.RawMessage(`"updated"`),
			}}},
		},
	})
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 || blocks[2]["type"] != protocol.ContentBlockTypeWorkGraphArtifact {
		t.Fatalf("structured runtime command artifact blocks = %#v", blocks)
	}
}

func TestProcessorAddsWorkGraphArtifactForUnquotedManagedArguments(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test", AgentID: "nexus",
		RoundID: "round-workgraph-artifact", ParentID: "round-workgraph-artifact",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
				ID: "tool-workgraph-unquoted", Name: "Bash",
				Input: json.RawMessage(`{"command":"\"${NEXUS_COMMAND_PATH}\" --json execution invoke --operation get_workgraph_preview --request-id get-preview-1234"}`),
			}},
		}},
	})
	payload := `{"domain":"execution","action":"invoke","operation":"get_workgraph_preview","is_error":false,"data":{"preview_id":"preview-1","head_revision":1,"selected_revision":1,"versions":[{"revision":1}],"preview":{"preview_id":"preview-1","slash_name":"briefing","title":"协作简报","source_execution_id":"execution-1","source_session_key":"agent:nexus:ws:dm:test","objective":"形成简报","nodes":[{"logical_key":"draft","role":"key","kind":"produce","subject":"起草","objective":"起草简报","deliverable":"简报","required":true,"terminal":true,"position":0}],"expires_at":"2026-08-22T00:00:00Z"}}}`
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolResultBlock{
				ToolUseID: "tool-workgraph-unquoted", Content: json.RawMessage(strconv.Quote(payload)),
			}},
		}},
	})
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 || blocks[2]["type"] != protocol.ContentBlockTypeWorkGraphArtifact {
		t.Fatalf("unquoted managed command artifact blocks = %#v", blocks)
	}
}

func TestProcessorRejectsWorkGraphPayloadFromUnmanagedCommand(t *testing.T) {
	processor := NewProcessor(MessageContext{SessionKey: "session", AgentID: "nexus", RoundID: "round"}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolUseBlock{
				ID: "tool-unmanaged", Name: "Bash", Input: json.RawMessage(`{"command":"echo fake"}`),
			}},
		}},
	})
	payload := `{"domain":"execution","action":"invoke","operation":"extract_workgraph_preview","is_error":false,"data":{"preview":{"preview_id":"fake","nodes":[{}]}}}`
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{Message: sdkprotocol.ConversationEnvelope{
			Content: []sdkprotocol.ContentBlock{sdkprotocol.ToolResultBlock{
				ToolUseID: "tool-unmanaged", Content: json.RawMessage(strconv.Quote(payload)),
			}},
		}},
	})
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("unmanaged payload created an artifact: %#v", blocks)
	}
}
