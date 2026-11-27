// model_probe_test.go 覆盖模型探测、检查规则与实机探测测试。
package provider

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
				if r.URL.Path != expected && r.URL.Path != "/embeddings" {
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
				if receipt := probeTestToolReceipt(payload); receipt != "" {
					text = receipt
				} else if imageData != "" {
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
		output := []any{map[string]any{"type": "reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": "test reasoning"}}}}
		if code != "" {
			output = append(output, map[string]any{"type": "function_call", "call_id": "call-1", "name": modelProbeToolName, "arguments": fmt.Sprintf(`{"code":%q}`, code)})
		} else {
			output = append(output, map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text}}})
		}
		return map[string]any{"status": "completed", "output": output}
	default:
		message := map[string]any{"role": "assistant", "content": text, "reasoning_content": "test reasoning"}
		if code != "" {
			message["tool_calls"] = []any{map[string]any{"id": "call-1", "type": "function", "function": map[string]any{"name": modelProbeToolName, "arguments": fmt.Sprintf(`{"code":%q}`, code)}}}
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
			result := service.checkVision(context.Background(), providerstore.Entity{ProviderKind: ProviderKindLLM, BaseURL: server.URL, APIFormat: APIFormatChatCompletions}, providerstore.ModelEntity{ModelID: "kimi-k2.6", ProviderOptionsJSON: "{}"})
			if tc.denied {
				if result.State != "unsupported" {
					t.Fatalf("missing explicit denial: %+v", result)
				}
			} else if result.State == "supported" || result.State == "unsupported" {
				t.Fatalf("unknown became a verdict: %+v", result)
			}
		})
	}
}

// probeTestToolReceipt reads only a tool-result block, never the original prompt.
func probeTestToolReceipt(payload map[string]any) string {
	var visit func(any) string
	visit = func(value any) string {
		switch item := value.(type) {
		case []any:
			for _, child := range item {
				if result := visit(child); result != "" {
					return result
				}
			}
		case map[string]any:
			if item["role"] == "tool" || item["type"] == "tool_result" || item["type"] == "function_call_output" {
				raw, _ := item["content"].(string)
				if item["type"] == "function_call_output" {
					raw, _ = item["output"].(string)
				}
				var result struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				}
				_ = json.Unmarshal([]byte(raw), &result)
				if len(result.Content) > 0 {
					return result.Content[0].Text
				}
			}
			for _, key := range []string{"messages", "input", "content"} {
				if result := visit(item[key]); result != "" {
					return result
				}
			}
		}
		return ""
	}
	return visit(payload)
}

func TestEmbeddingProbeRejectsPlaceholders(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid reordered", `{"data":[{"index":1,"embedding":[0.2,0.1]},{"index":0,"embedding":[0.1,0.2]}]}`, true},
		{"duplicate indices", `{"data":[{"index":0,"embedding":[1]},{"index":0,"embedding":[2]}]}`, false},
		{"zero", `{"data":[{"index":0,"embedding":[0,0]},{"index":1,"embedding":[1,2]}]}`, false},
		{"constant", `{"data":[{"index":0,"embedding":[1,2]},{"index":1,"embedding":[1,2]}]}`, false},
		{"dimension", `{"data":[{"index":0,"embedding":[1]},{"index":1,"embedding":[1,2]}]}`, false},
		{"nonfinite", `{"data":[{"index":0,"embedding":[1e999]},{"index":1,"embedding":[2]}]}`, false},
		{"missing", `{"data":[]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validEmbeddingProbe([]byte(tc.body)); got != tc.valid {
				t.Fatalf("valid=%v", got)
			}
		})
	}
}

func TestImageProbeVerifiesPixelsAndEditingSemantics(t *testing.T) {
	for _, mode := range []string{"generation", "edit", "unchanged", "unrelated", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			service := &Service{}
			calls := 0
			service.SetImageProbeAdapter(func(_ context.Context, request ImageProbeRequest) ([]byte, error) {
				calls++
				if mode == "corrupt" {
					return []byte("not an image"), nil
				}
				if !request.Edit {
					data, _, _ := newVisionProbeImage()
					return base64.StdEncoding.DecodeString(data)
				}
				original, err := png.Decode(bytes.NewReader(request.Image))
				if err != nil {
					t.Fatal(err)
				}
				canvas := image.NewRGBA(original.Bounds())
				draw.Draw(canvas, canvas.Bounds(), original, image.Point{}, draw.Src)
				switch mode {
				case "edit":
					draw.Draw(canvas, image.Rect(68, 68, 124, 124), image.NewUniform(color.RGBA{255, 0, 255, 255}), image.Point{}, draw.Src)
				case "unrelated":
					draw.Draw(canvas, canvas.Bounds(), image.Black, image.Point{}, draw.Src)
				}
				var output bytes.Buffer
				_ = png.Encode(&output, canvas)
				return output.Bytes(), nil
			})
			capability := "image_editing"
			if mode == "generation" {
				capability = "image_output"
			}
			result := service.checkImage(context.Background(), providerstore.Entity{ProviderKind: ProviderKindImageGeneration, APIFormat: APIFormatOpenAIImageGeneration, PresetKey: presetCustom}, providerstore.ModelEntity{ModelID: "probe-image", ProviderOptionsJSON: "{}"}, capability)
			want := "unknown"
			if mode == "generation" || mode == "edit" {
				want = "supported"
			}
			if result.State != want || calls != 1 {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
		})
	}
}

func TestToolsRequireReturnedReceiptAcrossProtocols(t *testing.T) {
	for _, format := range []string{APIFormatChatCompletions, APIFormatResponses, APIFormatAnthropicMessages} {
		t.Run(format, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var payload map[string]any
				_ = json.NewDecoder(r.Body).Decode(&payload)
				if probeTestToolReceipt(payload) != "" {
					_ = json.NewEncoder(w).Encode(probeTestResponse(format, "I called a tool.", ""))
					return
				}
				prompt, _ := probeTestInput(payload, format)
				// The initial challenge is visible, but the final receipt is not.
				var code string
				for i := 0; i+32 <= len(prompt); i++ {
					if i >= 10 && prompt[i-10:i] == "with code " {
						code = prompt[i : i+32]
						break
					}
				}
				_ = json.NewEncoder(w).Encode(probeTestResponse(format, "", code))
			}))
			defer server.Close()
			service := &Service{client: server.Client()}
			result := service.checkTools(context.Background(), providerstore.Entity{ProviderKind: ProviderKindLLM, APIFormat: format, BaseURL: server.URL}, providerstore.ModelEntity{ModelID: "unknown", ProviderOptionsJSON: "{}"})
			if result.State != "unknown" || result.Reason != "tool_result_unverified" || calls != 2 {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
		})
	}
}

func TestReasoningRequiresStructuredEvidence(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"choices":[{"message":{"role":"assistant","content":"I reasoned deeply"}}]}`, false},
		{`{"choices":[{"message":{"role":"assistant","content":"1830"}}],"usage":{"completion_tokens_details":{"reasoning_tokens":8}}}`, true},
		{`{"error":{"message":"failed"},"choices":[{"message":{"role":"assistant","reasoning_content":"fake"}}]}`, false},
	} {
		response := parseProbeResponse([]byte(tc.body), APIFormatChatCompletions)
		if response.Reasoning != tc.want {
			t.Fatalf("response=%+v", response)
		}
	}
}

func TestFailedRecheckRetainsVerifiedCapabilityAndRecordsFailure(t *testing.T) {
	deny := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deny {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_, data := probeTestInput(payload, APIFormatResponses)
		_ = json.NewEncoder(w).Encode(probeTestResponse(APIFormatResponses, probeTestReadImage(t, data), ""))
	}))
	defer server.Close()
	service, _ := newTestService(t)
	ctx := context.Background()
	record := createConfigurationVersionTestProvider(t, service, ctx, "recheck", server.URL)
	first, err := service.TestModelCapability(ctx, record.Provider, "private-vision-alias", "vision", nil, false)
	if err != nil || first.CapabilityResults["vision"].State != "supported" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	deny = true
	second, err := service.TestModelCapability(ctx, record.Provider, "private-vision-alias", "vision", nil, false)
	if err != nil || second.Success || second.CapabilityResults["vision"].Reason != "quota_or_rate_limit" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	current, err := service.Get(ctx, record.Provider)
	if err != nil {
		t.Fatal(err)
	}
	model := current.Models[0]
	if model.CapabilitiesAuto.Vision == nil || !*model.CapabilitiesAuto.Vision || model.CapabilityTests["vision"].State != "error" {
		t.Fatalf("model=%+v", model)
	}
}

func TestEmbeddingProbeEndpoints(t *testing.T) {
	for _, tc := range []struct{ base, want string }{
		{"https://example.test/v1", "https://example.test/v1/embeddings"},
		{"https://example.test/v1/chat/completions?mode=test", "https://example.test/v1/embeddings?mode=test"},
		{"https://example.test/v1/responses", "https://example.test/v1/embeddings"},
		{"https://example.test/v1/embeddings/", "https://example.test/v1/embeddings"},
		{"https://sample.openai.azure.com", "https://sample.openai.azure.com/openai/v1/embeddings"},
		{"https://sample.openai.azure.com/openai/deployments/my-deploy/chat/completions?api-version=2024-10-21", "https://sample.openai.azure.com/openai/deployments/my-deploy/embeddings?api-version=2024-10-21"},
	} {
		if got := embeddingProbeEndpoint(providerstore.Entity{BaseURL: tc.base}); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.base, got, tc.want)
		}
	}
}

func TestAllCapabilityCheckPreservesDisabledModelAndSelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/embeddings" {
			_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,2]},{"index":1,"embedding":[3,4]}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`))
	}))
	defer server.Close()
	service, _ := newTestService(t)
	ctx := context.Background()
	created := createConfigurationVersionTestProvider(t, service, ctx, "probe-all", server.URL)
	if _, err := service.UpdateModel(ctx, created.Provider, "disabled-model", UpdateModelInput{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	before, _ := service.Get(ctx, created.Provider)
	result, err := service.TestModelCapability(ctx, created.Provider, "disabled-model", "all", &before.ConfigurationVersion, false)
	if err != nil || len(result.CapabilityResults) != 7 {
		t.Fatalf("all checks: %+v %v", result, err)
	}
	after, err := service.Get(ctx, created.Provider)
	if err != nil || after.Models[0].Enabled || after.Models[0].IsDefault != before.Models[0].IsDefault {
		t.Fatalf("selection changed: %+v %v", after, err)
	}
	if after.Models[0].CapabilitiesAuto.Embedding == nil || !*after.Models[0].CapabilitiesAuto.Embedding {
		t.Fatal("embedding not tested independently")
	}
	if len(after.Models[0].CapabilityTests) != 7 {
		t.Fatal("missing independent attempts")
	}
}

func TestParallelCapabilityObservationsMergeOnlyUnchangedModels(t *testing.T) {
	for _, sameModel := range []bool{false, true} {
		t.Run(fmt.Sprint(sameModel), func(t *testing.T) {
			release := make(chan struct{})
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				<-release
				_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`))
			}))
			defer server.Close()
			ctx := context.Background()
			service, _ := newTestService(t)
			record := createConfigurationVersionTestProvider(t, service, ctx, "parallel-probes", server.URL)
			for _, id := range []string{"a", "b"} {
				if _, err := service.UpdateModel(ctx, record.Provider, id, UpdateModelInput{Enabled: false}); err != nil {
					t.Fatal(err)
				}
			}
			errorsCh := make(chan error, 2)
			for _, id := range []string{"a", map[bool]string{false: "b", true: "a"}[sameModel]} {
				go func(id string) {
					_, err := service.TestModelCapability(ctx, record.Provider, id, "all", nil, false)
					errorsCh <- err
				}(id)
			}
			deadline := time.Now().Add(3 * time.Second)
			for requests.Load() < 10 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			close(release)
			first, second := <-errorsCh, <-errorsCh
			if sameModel {
				if (first == nil) == (second == nil) || (first != nil && !errors.Is(first, ErrConfigurationVersionConflict)) || (second != nil && !errors.Is(second, ErrConfigurationVersionConflict)) {
					t.Fatalf("same model must reject stale evidence: %v / %v", first, second)
				}
			} else {
				if first != nil || second != nil {
					t.Fatalf("independent models conflicted: %v / %v", first, second)
				}
			}
		})
	}
}

func TestLiveConfiguredModelCapabilities(t *testing.T) {
	path := os.Getenv("NEXUS_PROVIDER_LIVE_DB")
	targets := os.Getenv("NEXUS_PROVIDER_LIVE_TARGETS")
	if path == "" || targets == "" {
		t.Skip("explicit local DB and targets required for live requests")
	}
	databaseURL := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal("open read-only local configuration failed")
	}
	defer db.Close()
	owner := os.Getenv("NEXUS_PROVIDER_LIVE_OWNER")
	if owner == "" {
		owner = "__system__"
	}
	for _, target := range strings.Split(targets, ",") {
		provider, modelID, ok := strings.Cut(strings.TrimSpace(target), "/")
		if !ok || provider == "" || modelID == "" {
			t.Fatal("targets must be exact provider/model pairs")
		}
		t.Run(target, func(t *testing.T) {
			var token, baseURL, format string
			err := db.QueryRow(`SELECT p.auth_token, p.base_url, p.api_format FROM provider p JOIN provider_models m ON m.provider_id=p.id
				WHERE p.owner_user_id=? AND p.provider=? AND m.model_id=? AND p.enabled=1 AND m.enabled=1 AND p.provider_kind='llm'`, owner, provider, modelID).Scan(&token, &baseURL, &format)
			if err != nil {
				t.Fatal("enabled local target not found")
			}
			service, _ := newTestService(t)
			service.SetHTTPClient(&http.Client{Timeout: 30 * time.Second, Transport: http.DefaultTransport})
			ctx := context.Background()
			// Custom routing avoids preset-specific recommendations. Read probe evidence
			// directly below so exact-ID catalog defaults cannot masquerade as live results.
			record, err := service.Create(ctx, CreateInput{Provider: "live-probe", PresetKey: presetCustom, APIFormat: format, AuthToken: token, BaseURL: baseURL, Enabled: true})
			if err != nil {
				t.Fatal("isolated provider setup failed")
			}
			result, err := service.TestModel(ctx, record.Provider, modelID)
			if err != nil {
				t.Fatal("live probe persistence failed")
			}
			if !result.Success {
				t.Fatalf("live connectivity failed: %s", result.Error)
			}
			current, err := service.Get(ctx, record.Provider)
			if err != nil || len(current.Models) != 1 {
				t.Fatal("probe result missing")
			}
			item, err := service.requireProvider(ctx, record.Provider)
			if err != nil {
				t.Fatal("probe provider missing")
			}
			stored, err := service.getModelByID(ctx, item.ID, modelID)
			if err != nil || stored == nil {
				t.Fatal("stored model missing")
			}
			probe := currentModelProbe(*item, *stored)
			if probe == nil {
				t.Fatal("configuration-bound live evidence missing")
			}
			for key, attempt := range probe.Attempts {
				t.Logf("%s: %s (%s)", key, attempt.State, attempt.Reason)
			}
			caps := probe.Capabilities
			state := func(value *bool) string {
				if value == nil {
					return "unknown"
				}
				if *value {
					return "supported"
				}
				return "unsupported"
			}
			t.Logf("text=%s vision=%s tools=%s reasoning=%s image_output=%s image_editing=%s embedding=%s", state(caps.TextOutput), state(caps.Vision), state(caps.ToolCalling), state(caps.Reasoning), state(caps.ImageOutput), state(caps.ImageEditing), state(caps.Embedding))
			if caps.TextOutput == nil || !*caps.TextOutput {
				t.Error("no text capability confirmed")
			}
		})
	}
}
