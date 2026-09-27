// INPUT: Exact chat route plus image, reasoning or two-turn tool challenge.
// OUTPUT: Positive evidence only after the capability-specific challenge completes.
// POS: Independent chat capability detectors; no user history or business tools.
package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	sdktools "github.com/nexus-research-lab/nexus-agent-sdk-bridge/tools"
	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func (s *Service) checkVision(ctx context.Context, item providerstore.Entity, model providerstore.ModelEntity) CapabilityProbeResult {
	if item.ProviderKind != ProviderKindLLM {
		return probeResult("unknown", "route_unavailable")
	}
	data, answer, err := newVisionProbeImage()
	if err != nil {
		return probeResult("error", "fixture_failed")
	}
	payload, _ := capabilityProbePayload(item, model.ModelID, visionProbePrompt, data, "")
	status, body, err := s.sendProbeRequest(ctx, item, applyProbeOptions(payload, model))
	if failure := probeFailure(status, body, err, "vision"); failure != nil {
		return *failure
	}
	response := parseProbeResponse(body, item.APIFormat)
	if response.Valid && strings.Join(strings.Fields(response.Text), "") == answer {
		return probeResult("supported", "image_challenge")
	}
	return probeResult("unknown", "image_answer_unverified")
}

func (s *Service) checkReasoning(ctx context.Context, item providerstore.Entity, model providerstore.ModelEntity) CapabilityProbeResult {
	if item.ProviderKind != ProviderKindLLM {
		return probeResult("unknown", "route_unavailable")
	}
	raw, _ := capabilityProbePayload(item, model.ModelID, "Compute 37 * 49 + 17 and verify the result. Give the answer.", "", "")
	var payload map[string]any
	_ = json.Unmarshal(applyProbeOptions(raw, model), &payload)
	switch item.APIFormat {
	case APIFormatResponses:
		if _, ok := payload["reasoning"]; !ok {
			payload["reasoning"] = map[string]any{"effort": "low", "summary": "auto"}
		}
	case APIFormatAnthropicMessages:
		if _, ok := payload["thinking"]; !ok {
			payload["thinking"] = map[string]any{"type": "enabled", "budget_tokens": 1024}
		}
		payload["max_tokens"] = 2048
	default:
		if _, ok := payload["reasoning_effort"]; !ok {
			payload["reasoning_effort"] = "low"
		}
	}
	raw, _ = json.Marshal(payload)
	status, body, err := s.sendProbeRequest(ctx, item, raw)
	if failure := probeFailure(status, body, err, "reasoning"); failure != nil {
		return *failure
	}
	response := parseProbeResponse(body, item.APIFormat)
	if response.Valid && response.Reasoning {
		return probeResult("supported", "structured_reasoning")
	}
	return probeResult("unknown", "no_reasoning_evidence")
}

func probeNonce() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func (s *Service) checkTools(ctx context.Context, item providerstore.Entity, model providerstore.ModelEntity) CapabilityProbeResult {
	if item.ProviderKind != ProviderKindLLM {
		return probeResult("unknown", "route_unavailable")
	}
	challenge, err := probeNonce()
	if err != nil {
		return probeResult("error", "fixture_failed")
	}
	prompt := "Call " + modelProbeToolName + " exactly once with code " + challenge + ". After the tool returns, reply with exactly its returned receipt and nothing else."
	payload, _ := capabilityProbePayload(item, model.ModelID, prompt, "", challenge)
	payload = applyProbeOptions(payload, model)
	status, body, err := s.sendProbeRequest(ctx, item, payload)
	if failure := probeFailure(status, body, err, "tool_calling"); failure != nil {
		return *failure
	}
	response := parseProbeResponse(body, item.APIFormat)
	if !response.Valid || len(response.Tools) != 1 {
		return probeResult("unknown", "tool_call_unverified")
	}
	call := response.Tools[0]
	if !call.ValidArguments || call.ID == "" || call.Name != modelProbeToolName || call.Code != challenge {
		return probeResult("unknown", "tool_arguments_unverified")
	}
	// This receipt does not exist in the initial prompt. It is created by the same
	// SDK MCP handler mechanism used for Nexus tools, with no business authority.
	receipt := ""
	server := sdktools.CreateSDKMCPServer(sdktools.SDKMCPServerOptions{Name: "nexus_capability_probe", Tools: []sdktools.Tool{
		sdktools.New(modelProbeToolName, "Return an ephemeral receipt.", probeToolSchema(), func(_ context.Context, input map[string]any, _ *sdktools.Context) (sdktools.Result, error) {
			if len(input) != 1 || input["code"] != challenge {
				return sdktools.Result{}, fmt.Errorf("invalid probe arguments")
			}
			var err error
			receipt, err = probeNonce()
			return sdktools.Text(receipt), err
		}),
	}})
	result, err := server.HandleMessage(ctx, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": call.Name, "arguments": map[string]any{"code": call.Code}}})
	if err != nil || result["error"] != nil || receipt == "" {
		return probeResult("error", "tool_execution_failed")
	}
	toolResult, _ := json.Marshal(result["result"])
	followup, err := probeToolFollowup(item.APIFormat, payload, body, call, string(toolResult))
	if err != nil {
		return probeResult("unknown", "tool_result_unverified")
	}
	status, body, err = s.sendProbeRequest(ctx, item, followup)
	if failure := probeFailure(status, body, err, "tool_calling"); failure != nil {
		return *failure
	}
	final := parseProbeResponse(body, item.APIFormat)
	if final.Valid && len(final.Tools) == 0 && probeContainsReceipt(final.Text, receipt) {
		return probeResult("supported", "tool_round_trip")
	}
	return probeResult("unknown", "tool_result_unverified")
}

func probeToolSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"code": map[string]any{"type": "string"}}, "required": []string{"code"}, "additionalProperties": false}
}

// Preserve the provider's assistant blocks, call IDs and thinking signatures.
func probeToolFollowup(format string, request, response []byte, call probeToolCall, result string) ([]byte, error) {
	var payload, wire map[string]any
	if err := json.Unmarshal(request, &payload); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(response, &wire); err != nil {
		return nil, err
	}
	switch format {
	case APIFormatResponses:
		input, _ := payload["input"].([]any)
		output, _ := wire["output"].([]any)
		input = append(input, output...)
		payload["input"] = append(input, map[string]any{"type": "function_call_output", "call_id": call.ID, "output": result})
	case APIFormatAnthropicMessages:
		messages, _ := payload["messages"].([]any)
		payload["messages"] = append(messages, map[string]any{"role": "assistant", "content": wire["content"]}, map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": call.ID, "content": result}}})
	default:
		choices, _ := wire["choices"].([]any)
		if len(choices) == 0 {
			return nil, fmt.Errorf("missing assistant")
		}
		choice, _ := choices[0].(map[string]any)
		messages, _ := payload["messages"].([]any)
		payload["messages"] = append(messages, choice["message"], map[string]any{"role": "tool", "tool_call_id": call.ID, "content": result})
	}
	return json.Marshal(payload)
}

// Wrapping prose/Markdown is allowed; an exact unpredictable receipt token still
// proves the model consumed the real result. Partial or extended tokens do not.
func probeContainsReceipt(text, receipt string) bool {
	if receipt == "" {
		return false
	}
	for _, token := range strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) }) {
		if token == receipt {
			return true
		}
	}
	return false
}
