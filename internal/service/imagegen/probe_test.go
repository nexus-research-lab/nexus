// INPUT: Synthetic in-memory fixtures and real production image protocol adapters.
// OUTPUT: Generation/edit probes share routing, uploads and output retrieval without writes/retries.
// POS: Adapter integration coverage replacing duplicated Provider image-test payloads.
package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestProbeUsesProductionImageAdapters(t *testing.T) {
	fixture := []byte("synthetic-fixture")
	for _, format := range []string{providercfg.APIFormatOpenAIImageGeneration, providercfg.APIFormatDashScopeImageGeneration, providercfg.APIFormatModelScopeImageGeneration} {
		for _, edit := range []bool{false, true} {
			if edit && format == providercfg.APIFormatModelScopeImageGeneration {
				continue
			}
			t.Run(format+map[bool]string{false: "/generate", true: "/edit"}[edit], func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.URL.Path == "/result.png" {
						_, _ = w.Write(fixture)
						return
					}
					if r.Header.Get("Authorization") != "Bearer probe-key" {
						t.Error("missing credentials")
					}
					if r.Method == "GET" {
						if !strings.HasSuffix(r.URL.Path, "/tasks/task-1") {
							t.Errorf("wrong task path %s", r.URL.Path)
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"task_status": "SUCCEED", "output_images": []string{"http://" + r.Host + "/result.png"}})
						return
					}
					if format == providercfg.APIFormatOpenAIImageGeneration && edit {
						if !strings.HasSuffix(r.URL.Path, "/images/edits") {
							t.Error("wrong edit path")
						}
						file, _, err := r.FormFile("image")
						if err != nil {
							t.Error(err)
							return
						}
						defer file.Close()
						uploaded, _ := io.ReadAll(file)
						if !bytes.Equal(uploaded, fixture) {
							t.Error("image upload changed")
						}
						if r.FormValue("prompt") != "probe instruction" {
							t.Error("prompt lost")
						}
					} else {
						var body map[string]any
						_ = json.NewDecoder(r.Body).Decode(&body)
						if body["model"] != "probe-model" {
							t.Error("model lost")
						}
						if format == providercfg.APIFormatDashScopeImageGeneration {
							raw, _ := json.Marshal(body)
							if !strings.HasSuffix(r.URL.Path, dashScopeGenerationPath) {
								t.Error("wrong DashScope path")
							}
							if edit && !bytes.Contains(raw, []byte(base64.StdEncoding.EncodeToString(fixture))) {
								t.Error("edit input lost")
							}
							_ = json.NewEncoder(w).Encode(map[string]any{"output": map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": []any{map[string]any{"image": "http://" + r.Host + "/result.png"}}}}}}})
							return
						}
						if format == providercfg.APIFormatModelScopeImageGeneration {
							if r.Header.Get("X-ModelScope-Async-Mode") != "true" {
								t.Error("missing async header")
							}
							_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "task-1"})
							return
						}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"url": "http://" + r.Host + "/result.png"}}})
				}))
				defer server.Close()
				service := NewService(nil, t.TempDir())
				service.client = server.Client()
				result, err := service.ProbeImage(context.Background(), providercfg.ImageProbeRequest{Config: providercfg.ImageConfig{APIFormat: format, BaseURL: server.URL, AuthToken: "probe-key", Model: "probe-model"}, Edit: edit, Image: fixture, Prompt: "probe instruction"})
				if err != nil || !bytes.Equal(result, fixture) {
					t.Fatalf("result=%q err=%v", result, err)
				}
				if calls < 2 {
					t.Fatal("output URL was not downloaded")
				}
			})
		}
	}
}

func TestImageProbeDoesNotRetryAndClassifiesDenial(t *testing.T) {
	for _, tc := range []struct {
		status              int
		code, state, reason string
	}{
		{429, "", "error", "quota_or_rate_limit"},
		{500, "image_generation_not_supported", "error", "image_request_failed"},
		{400, "image_generation_not_supported", "unsupported", "explicit_protocol_denial"},
		{400, "invalid_request", "error", "image_request_failed"},
	} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			w.WriteHeader(tc.status)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": tc.code}})
		}))
		service := NewService(nil, t.TempDir())
		service.client = server.Client()
		_, err := service.ProbeImage(context.Background(), providercfg.ImageProbeRequest{Config: providercfg.ImageConfig{BaseURL: server.URL, APIFormat: providercfg.APIFormatOpenAIImageGeneration, Model: "probe"}, Prompt: "test"})
		server.Close()
		var failure *providercfg.ProbeExecutionError
		if !errors.As(err, &failure) || failure.Result.State != tc.state || failure.Result.Reason != tc.reason || calls != 1 {
			t.Fatalf("error=%v calls=%d", err, calls)
		}
	}
}
