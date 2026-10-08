// INPUT: Real protocol fixtures, corrupt media, incomplete tool rounds and failed rechecks.
// OUTPUT: Seven independent evidence standards and preservation of prior verified facts.
// POS: Capability probe semantic and persistence regression tests.
package provider

import (
	"bytes"
	"context"
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
	"sync/atomic"
	"testing"
	"time"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

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
