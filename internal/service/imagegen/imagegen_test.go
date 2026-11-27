// imagegen_test.go 覆盖图像生成服务、夹具、探测与外部 Provider 测试。
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type fakeProviderResolver struct {
	config   *providercfg.ImageConfig
	provider string
	model    string
}

func (f fakeProviderResolver) ResolveImageConfig(_ context.Context, _ string) (*providercfg.ImageConfig, error) {
	return f.config, nil
}

func (f *fakeProviderResolver) ResolveImageModelConfig(_ context.Context, provider string, model string) (*providercfg.ImageConfig, error) {
	f.provider = provider
	f.model = model
	return f.config, nil
}

func newImagegenWorkspace(t *testing.T) string {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	workspacePath := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, authctx.SystemUserID),
		"agent-1",
	)
	if err := os.MkdirAll(workspacePath, 0o700); err != nil {
		t.Fatalf("创建图片测试 workspace 失败: %v", err)
	}
	return workspacePath
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
		t.Fatalf("write test png: %v", err)
	}
}

func TestGenerateImageSupportsAzureDeploymentURL(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		expectedPath := "/openai/deployments/gpt-image-2/images/generations"
		if request.URL.Path != expectedPath {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("api-version") != "2024-02-01" {
			t.Fatalf("missing api-version: %s", request.URL.RawQuery)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := body["model"]; ok {
			t.Fatalf("azure deployment request must not include model: %+v", body)
		}
		if body["output_compression"].(float64) != 100 {
			t.Fatalf("unexpected output_compression: %+v", body)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}},
		})
	}))
	defer server.Close()

	compression := 100
	workspacePath := newImagegenWorkspace(t)
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "azure-image",
		AuthToken: "azure-token",
		BaseURL:   server.URL + "/openai/deployments/gpt-image-2?api-version=2024-02-01",
		Model:     "gpt-image-2",
	}}, "")
	result, _, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:            "A photograph of a red fox in an autumn forest",
		WorkspacePath:     workspacePath,
		Quality:           "low",
		OutputFormat:      "png",
		OutputCompression: &compression,
		FileName:          "fox",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if result.Path != "output/imagegen/fox.png" {
		t.Fatalf("unexpected path: %s", result.Path)
	}
}

func TestEditImageSupportsAzureMultipartAPI(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/openai/deployments/gpt-image-2/images/edits" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.URL.Query().Get("api-version") != "2024-02-01" {
			t.Fatalf("missing api-version: %s", request.URL.RawQuery)
		}
		reader, err := request.MultipartReader()
		if err != nil {
			t.Fatalf("expected multipart request: %v", err)
		}
		seen := map[string]string{}
		for {
			part, partErr := reader.NextPart()
			if partErr == io.EOF {
				break
			}
			if partErr != nil {
				t.Fatalf("read multipart: %v", partErr)
			}
			data, _ := io.ReadAll(part)
			seen[part.FormName()] = string(data)
		}
		if seen["prompt"] != "Make this black and white" || seen["image"] == "" || seen["mask"] == "" {
			t.Fatalf("unexpected multipart fields: %+v", seen)
		}
		if _, ok := seen["model"]; ok {
			t.Fatalf("azure edit request must not include model: %+v", seen)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(imageBytes)}},
		})
	}))
	defer server.Close()

	workspacePath := newImagegenWorkspace(t)
	writeTestPNG(t, filepath.Join(workspacePath, "image_to_edit.png"))
	writeTestPNG(t, filepath.Join(workspacePath, "mask.png"))
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "azure-image",
		AuthToken: "azure-token",
		BaseURL:   server.URL + "/openai/deployments/gpt-image-2?api-version=2024-02-01",
		Model:     "gpt-image-2",
	}}, "")
	result, _, err := service.EditImage(context.Background(), EditInput{
		Prompt:        "Make this black and white",
		WorkspacePath: workspacePath,
		ImagePath:     "image_to_edit.png",
		MaskPath:      "mask.png",
		FileName:      "edited",
	})
	if err != nil {
		t.Fatalf("EditImage returned error: %v", err)
	}
	if result.Path != "output/imagegen/edited.png" {
		t.Fatalf("unexpected path: %s", result.Path)
	}
}

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
