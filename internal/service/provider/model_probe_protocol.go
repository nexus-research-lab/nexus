// INPUT: Exact supported chat protocol and synthetic text/image/tool challenges.
// OUTPUT: Protocol-specific requests and strictly parsed capability observations.
// POS: Provider capability probe wire formats; tool results preserve protocol identity and signed assistant blocks.
package provider

import (
	"encoding/json"
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const modelProbeMaxTokens = 1024
const modelProbeToolName = "report_capability_probe"
const visionProbePrompt = "Read the 3 by 3 color grid in the image, left to right, top to bottom. Encode red as 1, green as 2, blue as 3, yellow as 4. Reply with exactly nine digits and nothing else. If you cannot see the image, reply UNKNOWN."

func capabilityProbePayload(item providerstore.Entity, modelID, prompt, imageData, toolCode string) ([]byte, error) {
	schema := probeToolSchema()
	payload := map[string]any{"model": modelID, "stream": false}
	switch item.APIFormat {
	case APIFormatAnthropicMessages:
		content := []any{map[string]any{"type": "text", "text": prompt}}
		if imageData != "" {
			content = append(content, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": "image/png", "data": imageData}})
		}
		payload["messages"] = []any{map[string]any{"role": "user", "content": content}}
		payload["max_tokens"] = modelProbeMaxTokens
		if toolCode != "" {
			payload["tools"] = []any{map[string]any{"name": modelProbeToolName, "description": "Validate the provided code and return an ephemeral receipt. No business action is executed.", "input_schema": schema}}
		}
	case APIFormatResponses:
		content := []any{map[string]any{"type": "input_text", "text": prompt}}
		if imageData != "" {
			content = append(content, map[string]any{"type": "input_image", "image_url": "data:image/png;base64," + imageData})
		}
		payload["input"] = []any{map[string]any{"role": "user", "content": content}}
		payload["max_output_tokens"] = modelProbeMaxTokens
		payload["store"] = false
		if toolCode != "" {
			payload["tools"] = []any{map[string]any{"type": "function", "name": modelProbeToolName, "description": "Validate the provided code and return an ephemeral receipt. No business action is executed.", "parameters": schema}}
		}
	default:
		content := []any{map[string]any{"type": "text", "text": prompt}}
		if imageData != "" {
			content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64," + imageData}})
		}
		payload["messages"] = []any{map[string]any{"role": "user", "content": content}}
		if usesMaxCompletionTokens(item) {
			payload["max_completion_tokens"] = modelProbeMaxTokens
		} else {
			payload["max_tokens"] = modelProbeMaxTokens
		}
		if toolCode != "" {
			payload["tools"] = []any{map[string]any{"type": "function", "function": map[string]any{"name": modelProbeToolName, "description": "Validate the provided code and return an ephemeral receipt. No business action is executed.", "parameters": schema}}}
		}
	}
	return json.Marshal(payload)
}

type probeToolCall struct {
	ValidArguments bool
	ID             string
	Name           string
	Code           string
}

type probeResponse struct {
	Valid     bool
	Text      string
	Reasoning bool
	Tools     []probeToolCall
}

func parseProbeResponse(body []byte, format string) probeResponse {
	var wire struct {
		Type    string           `json:"type"`
		Role    string           `json:"role"`
		Error   json.RawMessage  `json:"error"`
		Status  string           `json:"status"`
		Content []map[string]any `json:"content"`
		Output  []map[string]any `json:"output"`
		Choices []struct {
			Message struct {
				Role             string          `json:"role"`
				Content          json.RawMessage `json:"content"`
				Reasoning        string          `json:"reasoning"`
				ReasoningContent string          `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &wire) != nil || (len(wire.Error) > 0 && string(wire.Error) != "null") || wire.Status == "failed" {
		return probeResponse{}
	}
	result := probeResponse{}
	addTool := func(id, name string, input any) {
		var arguments map[string]any
		if text, ok := input.(string); ok {
			_ = json.Unmarshal([]byte(text), &arguments)
		} else {
			raw, _ := json.Marshal(input)
			_ = json.Unmarshal(raw, &arguments)
		}
		code, valid := arguments["code"].(string)
		result.Tools = append(result.Tools, probeToolCall{ID: id, Name: name, Code: code, ValidArguments: valid && len(arguments) == 1})
	}
	readBlocks := func(blocks []map[string]any) {
		for _, block := range blocks {
			switch block["type"] {
			case "text", "output_text":
				value, _ := block["text"].(string)
				result.Text += value
			case "thinking":
				value, _ := block["thinking"].(string)
				result.Reasoning = result.Reasoning || strings.TrimSpace(value) != ""
			case "tool_use":
				name, _ := block["name"].(string)
				id, _ := block["id"].(string)
				addTool(id, name, block["input"])
			}
		}
	}
	switch format {
	case APIFormatAnthropicMessages:
		if wire.Type != "message" || wire.Role != "assistant" {
			return result
		}
		result.Valid = true
		readBlocks(wire.Content)
	case APIFormatResponses:
		if wire.Status != "completed" && wire.Status != "incomplete" {
			return result
		}
		result.Valid = true
		for _, output := range wire.Output {
			switch output["type"] {
			case "message":
				raw, _ := json.Marshal(output["content"])
				var blocks []map[string]any
				_ = json.Unmarshal(raw, &blocks)
				readBlocks(blocks)
			case "reasoning":
				encrypted, _ := output["encrypted_content"].(string)
				result.Reasoning = result.Reasoning || strings.TrimSpace(encrypted) != ""
				summary, _ := output["summary"].([]any)
				for _, entry := range summary {
					block, _ := entry.(map[string]any)
					text, _ := block["text"].(string)
					result.Reasoning = result.Reasoning || strings.TrimSpace(text) != ""
				}
			case "function_call":
				name, _ := output["name"].(string)
				id, _ := output["call_id"].(string)
				addTool(id, name, output["arguments"])
			}
		}
	default:
		if len(wire.Choices) == 0 || wire.Choices[0].Message.Role != "assistant" {
			return result
		}
		message := wire.Choices[0].Message
		result.Valid = true
		if json.Unmarshal(message.Content, &result.Text) != nil {
			var blocks []map[string]any
			_ = json.Unmarshal(message.Content, &blocks)
			readBlocks(blocks)
		}
		result.Reasoning = strings.TrimSpace(message.ReasoningContent) != "" || strings.TrimSpace(message.Reasoning) != ""
		for _, tool := range message.ToolCalls {
			if tool.Type == "function" {
				addTool(tool.ID, tool.Function.Name, tool.Function.Arguments)
			}
		}
	}
	var usage struct {
		Usage struct {
			Completion struct {
				Reasoning int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
			Output struct {
				Reasoning int `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(body, &usage)
	result.Reasoning = result.Reasoning || usage.Usage.Completion.Reasoning > 0 || usage.Usage.Output.Reasoning > 0
	result.Text = strings.TrimSpace(result.Text)
	return result
}
