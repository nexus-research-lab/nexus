// INPUT: Real HTTP probe requests against local protocol-aware servers and isolated SQLite.
// OUTPUT: Verified capability projection, configuration fencing and unknown/error regression coverage.
// POS: Provider detection acceptance tests; no external credentials or paid model requests.
package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

// 未收录的私有别名用于确保这些结果来自实际协议探测，而非精确 ID 目录默认值。
func TestCapabilityProbeRoundTripAcrossProtocols(t *testing.T) {
	for _, format := range []string{APIFormatChatCompletions, APIFormatResponses, APIFormatAnthropicMessages} {
		t.Run(format, func(t *testing.T) {
			var visionCalls, toolCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`{"data":[{"id":"private-kimi-k2.6-alias"}]}`))
					return
				}
				expected := map[string]string{APIFormatChatCompletions: "/chat/completions", APIFormatResponses: "/responses", APIFormatAnthropicMessages: "/v1/messages"}[format]
				if r.URL.Path != expected {
					t.Errorf("path = %s", r.URL.Path)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				if payload["model"] != "private-kimi-k2.6-alias" {
					t.Errorf("wrong model: %v", payload["model"])
				}
				prompt, imageData := probeTestInput(payload, format)
				text, code := "pong", ""
				if imageData != "" {
					visionCalls.Add(1)
					text = probeTestReadImage(t, imageData)
					if strings.Contains(prompt, text) {
						t.Error("visual answer leaked into prompt")
					}
				} else if _, ok := payload["tools"]; ok {
					toolCalls.Add(1)
					prefix, suffix, found := strings.Cut(prompt, "with code ")
					if !found || !strings.Contains(prefix, modelProbeToolName) {
						t.Error("missing tool challenge")
					}
					code = strings.Split(suffix, ".")[0]
					text = ""
				}
				_ = json.NewEncoder(w).Encode(probeTestResponse(format, text, code))
			}))
			t.Cleanup(server.Close)
			ctx := context.Background()
			service, _ := newTestService(t)
			record, err := service.Create(ctx, CreateInput{Provider: "custom-probe", PresetKey: presetCustom, APIFormat: format, AuthToken: "local-test-key", BaseURL: server.URL, ModelsPath: "/models", Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = service.UpdateModel(ctx, record.Provider, "private-kimi-k2.6-alias", UpdateModelInput{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			result, err := service.TestModel(ctx, record.Provider, "private-kimi-k2.6-alias")
			if err != nil || !result.Success {
				t.Fatalf("test result: %+v, %v", result, err)
			}
			if visionCalls.Load() != 1 || toolCalls.Load() != 1 {
				t.Fatalf("probes: vision=%d tools=%d", visionCalls.Load(), toolCalls.Load())
			}
			current, err := service.Get(ctx, record.Provider)
			if err != nil {
				t.Fatal(err)
			}
			model := current.Models[0]
			for name, value := range map[string]*bool{"text": model.CapabilitiesAuto.TextOutput, "vision": model.CapabilitiesAuto.Vision, "tools": model.CapabilitiesAuto.ToolCalling, "reasoning": model.CapabilitiesAuto.Reasoning} {
				if value == nil || !*value {
					t.Errorf("%s not recognized: %+v", name, model.Guidance)
				}
			}
			if model.CapabilitiesAuto.ImageOutput != nil || model.CapabilitiesAuto.ImageEditing != nil || model.CapabilitiesAuto.Embedding != nil {
				t.Fatal("chat probes invented unrelated capabilities")
			}
			if model.Guidance.Sources["vision"] != "probe" || !model.Guidance.Eligibility[PurposeVision].Available {
				t.Fatalf("guidance = %+v", model.Guidance)
			}
			config, err := service.ResolveRuntimeConfigForRuntime(ctx, record.Provider, "private-kimi-k2.6-alias", "nxs")
			if err != nil || !config.Vision {
				t.Fatalf("runtime = %+v, %v", config, err)
			}
			// Refreshing an ID-only model list must not erase successful probes.
			fetched, err := service.FetchModels(ctx, record.Provider)
			if err != nil || fetched.Models[0].CapabilitiesAuto.Vision == nil || !*fetched.Models[0].CapabilitiesAuto.Vision {
				t.Fatalf("rediscovery lost probe: %+v %v", fetched, err)
			}
			// User denial remains authoritative, while Automatic still shows its result.
			updated, err := service.UpdateModel(ctx, record.Provider, "private-kimi-k2.6-alias", UpdateModelInput{Enabled: true, CapabilitiesOverride: ModelCapabilities{Vision: adviceBool(false)}})
			if err != nil || updated.Guidance.Eligibility[PurposeVision].Available || updated.CapabilitiesAuto.Vision == nil || !*updated.CapabilitiesAuto.Vision {
				t.Fatalf("override/auto conflated: %+v %v", updated, err)
			}
			// Same model name on a different key is not the same verified route.
			key := "another-local-key"
			current, _ = service.Get(ctx, record.Provider)
			current, err = service.PatchAtVersion(ctx, record.Provider, PatchInput{AuthToken: &key}, current.ConfigurationVersion)
			if err != nil {
				t.Fatal(err)
			}
			if current.Models[0].CapabilitiesAuto.Vision != nil {
				t.Fatal("old credential probe still active")
			}
		})
	}
}

func probeTestInput(payload map[string]any, format string) (prompt, data string) {
	var input any = payload["messages"]
	if format == APIFormatResponses {
		input = payload["input"]
	}
	if text, ok := input.(string); ok {
		return text, ""
	}
	messages, _ := input.([]any)
	if len(messages) == 0 {
		return "", ""
	}
	message, _ := messages[0].(map[string]any)
	if text, ok := message["content"].(string); ok {
		return text, ""
	}
	blocks, _ := message["content"].([]any)
	for _, value := range blocks {
		block, _ := value.(map[string]any)
		switch block["type"] {
		case "text", "input_text":
			prompt, _ = block["text"].(string)
		case "image":
			source, _ := block["source"].(map[string]any)
			data, _ = source["data"].(string)
		case "input_image":
			data, _ = block["image_url"].(string)
		case "image_url":
			source, _ := block["image_url"].(map[string]any)
			data, _ = source["url"].(string)
		}
	}
	return prompt, strings.TrimPrefix(data, "data:image/png;base64,")
}

// Decode pixels rather than copying any expected answer from the request text.
func probeTestReadImage(t *testing.T, encoded string) string {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Error(err)
		return ""
	}
	canvas, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Error(err)
		return ""
	}
	answer := ""
	for i := 0; i < 9; i++ {
		r, g, b, _ := canvas.At((i%3)*64+32, (i/3)*64+32).RGBA()
		switch {
		case r > 50000 && g > 50000:
			answer += "4"
		case r > g && r > b:
			answer += "1"
		case g > r && g > b:
			answer += "2"
		default:
			answer += "3"
		}
	}
	return answer
}

func probeTestResponse(format, text, code string) any {
	switch format {
	case APIFormatAnthropicMessages:
		blocks := []any{map[string]any{"type": "thinking", "thinking": "test reasoning"}}
		if code != "" {
			blocks = append(blocks, map[string]any{"type": "tool_use", "id": "call-1", "name": modelProbeToolName, "input": map[string]string{"code": code}})
		} else {
			blocks = append(blocks, map[string]any{"type": "text", "text": text})
		}
		return map[string]any{"type": "message", "role": "assistant", "content": blocks}
	case APIFormatResponses:
		output := []any{map[string]any{"type": "reasoning", "summary": []any{}}}
		if code != "" {
			output = append(output, map[string]any{"type": "function_call", "name": modelProbeToolName, "arguments": fmt.Sprintf(`{"code":%q}`, code)})
		} else {
			output = append(output, map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}}})
		}
		return map[string]any{"status": "completed", "output": output}
	default:
		message := map[string]any{"role": "assistant", "content": text, "reasoning_content": "test reasoning"}
		if code != "" {
			message["tool_calls"] = []any{map[string]any{"type": "function", "function": map[string]any{"name": modelProbeToolName, "arguments": fmt.Sprintf(`{"code":%q}`, code)}}}
		}
		return map[string]any{"choices": []any{map[string]any{"message": message}}}
	}
}

func TestProbeFailuresNeverInventUnsupportedCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		denied bool
	}{
		{"timeout-like", 504, `{"error":{"code":"vision_not_supported"}}`, false},
		{"auth", 401, `{"error":{"code":"vision_not_supported"}}`, false},
		{"quota", 429, `{"error":{"code":"quota_exceeded"}}`, false},
		{"invalid-parameter", 400, `{"error":{"code":"invalid_request_error","message":"vision problem"}}`, false},
		{"explicit", 400, `{"error":{"code":"vision_not_supported"}}`, true},
		{"wrong-answer", 200, `{"choices":[{"message":{"role":"assistant","content":"UNKNOWN"}}]}`, false},
		{"empty-success", 200, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			service := &Service{client: server.Client()}
			result := service.probeVision(context.Background(), providerstore.Entity{BaseURL: server.URL, APIFormat: APIFormatChatCompletions}, "kimi-k2.6")
			if tc.denied {
				if result.Vision == nil || *result.Vision {
					t.Fatalf("missing explicit denial: %+v", result)
				}
			} else if result.Vision != nil {
				t.Fatalf("unknown became a verdict: %+v", result)
			}
		})
	}
}

func TestProbeConfigurationFenceRejectsLateResults(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { close(started); <-release; _, _ = w.Write([]byte(`{}`)) }))
	defer server.Close()
	service, _ := newTestService(t)
	ctx := context.Background()
	created := createConfigurationVersionTestProvider(t, service, ctx, "probe-fence", server.URL)
	completed := make(chan error, 1)
	go func() {
		_, err := service.TestModelAtVersion(ctx, created.Provider, "kimi-k2.6", created.ConfigurationVersion)
		completed <- err
	}()
	<-started
	key := "rotated-key"
	_, err := service.PatchAtVersion(ctx, created.Provider, PatchInput{AuthToken: &key}, created.ConfigurationVersion)
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if err = <-completed; !errors.Is(err, ErrConfigurationVersionConflict) {
		t.Fatalf("late result: %v", err)
	}
	current, err := service.Get(ctx, created.Provider)
	if err != nil || len(current.Models) != 0 {
		t.Fatalf("stale test wrote model: %+v %v", current, err)
	}
}

func TestCapabilityEvidenceIdentityAndUnknownMerge(t *testing.T) {
	item := providerstore.Entity{ID: "p", OwnerUserID: "owner", Visibility: "private", PresetKey: presetCustom, ProviderKind: ProviderKindLLM, APIFormat: APIFormatChatCompletions, AuthToken: "test-key", BaseURL: "https://local.test"}
	model := providerstore.ModelEntity{ProviderID: "p", ModelID: "kimi-k2.6", ProviderOptionsJSON: `{}`}
	model.CapabilitiesAutoJSON = withModelProbe(`{"vision":true}`, modelProbeEvidence{Version: modelProbeVersion, Fingerprint: modelProbeFingerprint(item, model), TestedAt: time.Now(), Capabilities: ModelCapabilities{Vision: adviceBool(true)}})
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("current evidence lost")
	}
	if decodeModelAutoCapabilities(model.CapabilitiesAutoJSON).Vision != nil {
		t.Fatal("probe upgraded historical guesses")
	}
	for _, mutate := range []func(*providerstore.Entity, *providerstore.ModelEntity){
		func(p *providerstore.Entity, _ *providerstore.ModelEntity) { p.OwnerUserID = "other" },
		func(p *providerstore.Entity, _ *providerstore.ModelEntity) { p.ID = "other" },
		func(p *providerstore.Entity, _ *providerstore.ModelEntity) { p.BaseURL += "/other" },
		func(p *providerstore.Entity, _ *providerstore.ModelEntity) { p.APIFormat = APIFormatResponses },
		func(_ *providerstore.Entity, m *providerstore.ModelEntity) { m.ModelID = "other" },
		func(_ *providerstore.Entity, m *providerstore.ModelEntity) {
			m.ProviderOptionsJSON = `{"thinking":false}`
		},
	} {
		p, m := item, model
		mutate(&p, &m)
		if currentModelProbe(p, m) != nil {
			t.Fatal("evidence crossed configuration identity")
		}
	}
	item.DisplayName = "renamed"
	item.ConfigurationVersion++
	if currentModelProbe(item, model) == nil {
		t.Fatal("cosmetic change discarded proof")
	}
	merged := mergeObservedCapabilities(ModelCapabilities{Vision: adviceBool(true)}, ModelCapabilities{})
	if merged.Vision == nil || !*merged.Vision {
		t.Fatal("unknown erased success")
	}
}
