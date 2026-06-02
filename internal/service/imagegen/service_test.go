package imagegen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type fakeProviderResolver struct {
	config   *providercfg.ImageConfig
	provider string
	model    string
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
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "azure-image",
		AuthToken: "azure-token",
		BaseURL:   server.URL + "/openai/deployments/gpt-image-2?api-version=2024-02-01",
		Model:     "gpt-image-2",
	}})
	result, _, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:            "A photograph of a red fox in an autumn forest",
		WorkspacePath:     t.TempDir(),
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

	workspacePath := t.TempDir()
	writeTestPNG(t, filepath.Join(workspacePath, "image_to_edit.png"))
	writeTestPNG(t, filepath.Join(workspacePath, "mask.png"))
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "azure-image",
		AuthToken: "azure-token",
		BaseURL:   server.URL + "/openai/deployments/gpt-image-2?api-version=2024-02-01",
		Model:     "gpt-image-2",
	}})
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

func (f fakeProviderResolver) ResolveImageConfig(_ context.Context, _ string) (*providercfg.ImageConfig, error) {
	return f.config, nil
}

func (f *fakeProviderResolver) ResolveImageModelConfig(_ context.Context, provider string, model string) (*providercfg.ImageConfig, error) {
	f.provider = provider
	f.model = model
	return f.config, nil
}

type fakePreferencesService struct {
	prefs preferencessvc.Preferences
}

func (f fakePreferencesService) Get(_ context.Context, _ string) (preferencessvc.Preferences, error) {
	return f.prefs, nil
}

func TestResolveImageConfigUsesPreferenceDefaultModel(t *testing.T) {
	resolver := &fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "image-provider",
		AuthToken: "token",
		BaseURL:   "https://image.example.com/v1/images",
		Model:     "image-model",
	}}
	service := NewService(resolver)
	service.SetPreferences(fakePreferencesService{prefs: preferencessvc.Preferences{
		DefaultImageModelSelection: preferencessvc.ModelSelection{
			Provider: "image-provider",
			Model:    "image-model",
		},
	}})
	config, err := service.resolveImageConfig(context.Background(), "", "")
	if err != nil {
		t.Fatalf("解析图片默认模型失败: %v", err)
	}
	if config.Model != "image-model" || resolver.provider != "image-provider" || resolver.model != "image-model" {
		t.Fatalf("未使用默认生图模型: config=%+v provider=%s model=%s", config, resolver.provider, resolver.model)
	}
}

func TestResolveImageConfigUsesExplicitProviderModel(t *testing.T) {
	resolver := &fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "image-provider",
		AuthToken: "token",
		BaseURL:   "https://image.example.com/v1/images",
		Model:     "image-model",
	}}
	service := NewService(resolver)

	config, err := service.resolveImageConfig(context.Background(), "image-provider", "image-model")
	if err != nil {
		t.Fatalf("解析显式图片模型失败: %v", err)
	}
	if config.Model != "image-model" || resolver.provider != "image-provider" || resolver.model != "image-model" {
		t.Fatalf("未使用显式图片模型: config=%+v provider=%s model=%s", config, resolver.provider, resolver.model)
	}
}

func TestGenerateImageCallsOpenAICompatibleProviderAndWritesFile(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected auth: %q", request.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["model"] != "gpt-image-1" || body["prompt"] != "a clean product photo" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		if body["response_format"] != "b64_json" {
			t.Fatalf("provider_options 未透传到 OpenAI-compatible 请求体: %+v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": []map[string]any{{
				"b64_json":       base64.StdEncoding.EncodeToString(imageBytes),
				"revised_prompt": "revised",
			}},
		})
	}))
	defer server.Close()

	workspacePath := t.TempDir()
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "openai",
		AuthToken: "test-token",
		BaseURL:   server.URL + "/v1",
		Model:     "gpt-image-1",
		ProviderOptions: map[string]any{
			"response_format": "b64_json",
		},
	}})
	service.now = fixedNow

	result, payload, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:        "a clean product photo",
		WorkspacePath: workspacePath,
		FileName:      "hero-image",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if string(payload) != string(imageBytes) {
		t.Fatalf("payload mismatch")
	}
	if result.Path != "output/imagegen/hero-image.png" {
		t.Fatalf("unexpected path: %s", result.Path)
	}
	if result.MIMEType != "image/png" {
		t.Fatalf("unexpected mime: %s", result.MIMEType)
	}
	stored, err := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(result.Path)))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(stored) != string(imageBytes) {
		t.Fatalf("stored file mismatch")
	}
}

func TestGenerateImageCallsDoubaoSeedreamProviderAndWritesFile(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v3/images/generations":
			if request.Header.Get("Authorization") != "Bearer doubao-token" {
				t.Fatalf("unexpected auth: %q", request.Header.Get("Authorization"))
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body["model"] != "doubao-seedream-5-0-260128" || body["prompt"] != "A golden cat" {
				t.Fatalf("unexpected request body: %+v", body)
			}
			if body["size"] != "2K" || body["watermark"] != false || body["response_format"] != "url" {
				t.Fatalf("Doubao Seedream 默认参数不正确: %+v", body)
			}
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"data": []map[string]any{{
					"url": server.URL + "/doubao.png",
				}},
			})
		case "/doubao.png":
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write(imageBytes)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	workspacePath := t.TempDir()
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "doubao",
		AuthToken: "doubao-token",
		BaseURL:   server.URL + "/api/v3",
		Model:     "doubao-seedream-5-0-260128",
		ProviderOptions: map[string]any{
			"response_format": "url",
		},
	}})
	result, payload, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:        "A golden cat",
		WorkspacePath: workspacePath,
		FileName:      "doubao-cat",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if string(payload) != string(imageBytes) {
		t.Fatalf("payload mismatch")
	}
	if result.Provider != "doubao" || result.Model != "doubao-seedream-5-0-260128" || result.Size != "2K" {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	stored, err := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(result.Path)))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(stored) != string(imageBytes) {
		t.Fatalf("stored file mismatch")
	}
}

func TestGenerateImageCallsDashScopeProviderAndWritesFile(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/services/aigc/multimodal-generation/generation":
			if request.Header.Get("Authorization") != "Bearer dashscope-token" {
				t.Fatalf("unexpected auth: %q", request.Header.Get("Authorization"))
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body["model"] != "wan2.7-image-pro" {
				t.Fatalf("unexpected model: %+v", body)
			}
			input := body["input"].(map[string]any)
			messages := input["messages"].([]any)
			firstMessage := messages[0].(map[string]any)
			content := firstMessage["content"].([]any)
			firstContent := content[0].(map[string]any)
			if firstContent["text"] != "一只橘猫在阳光下打盹" {
				t.Fatalf("unexpected prompt content: %+v", content)
			}
			parameters := body["parameters"].(map[string]any)
			if parameters["size"] != "1024*1024" || parameters["watermark"] != false || parameters["thinking_mode"] != true {
				t.Fatalf("unexpected DashScope parameters: %+v", parameters)
			}
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"output": map[string]any{
					"finished": true,
					"choices": []map[string]any{{
						"finish_reason": "stop",
						"message": map[string]any{
							"role": "assistant",
							"content": []map[string]any{{
								"type":  "image",
								"image": server.URL + "/generated.png",
							}},
						},
					}},
				},
			})
		case "/generated.png":
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write(imageBytes)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	workspacePath := t.TempDir()
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "dashscope",
		APIFormat: providercfg.APIFormatDashScopeImageGeneration,
		AuthToken: "dashscope-token",
		BaseURL:   server.URL,
		Model:     "wan2.7-image-pro",
		ProviderOptions: map[string]any{
			"parameters": map[string]any{
				"thinking_mode": true,
			},
		},
	}})
	result, payload, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:        "一只橘猫在阳光下打盹",
		WorkspacePath: workspacePath,
		FileName:      "dashscope-cat",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if string(payload) != string(imageBytes) {
		t.Fatalf("payload mismatch")
	}
	if result.Provider != "dashscope" || result.Model != "wan2.7-image-pro" {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	stored, err := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(result.Path)))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(stored) != string(imageBytes) {
		t.Fatalf("stored file mismatch")
	}
}

func TestGenerateImageCallsModelScopeProviderAndWritesFile(t *testing.T) {
	imageBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/images/generations":
			if request.Header.Get("Authorization") != "Bearer modelscope-token" {
				t.Fatalf("unexpected auth: %q", request.Header.Get("Authorization"))
			}
			if request.Header.Get("X-ModelScope-Async-Mode") != "true" {
				t.Fatalf("unexpected async header: %q", request.Header.Get("X-ModelScope-Async-Mode"))
			}
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body["model"] != "Tongyi-MAI/Z-Image-Turbo" || body["prompt"] != "A golden cat" {
				t.Fatalf("unexpected request body: %+v", body)
			}
			if body["loras"] != "modelscope-lora" {
				t.Fatalf("provider_options 未透传到 ModelScope 请求体: %+v", body)
			}
			if _, ok := body["size"]; ok {
				t.Fatalf("ModelScope 默认尺寸不应作为额外字段发送: %+v", body)
			}
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{"task_id": "task-123"})
		case "/v1/tasks/task-123":
			if request.Header.Get("Authorization") != "Bearer modelscope-token" {
				t.Fatalf("unexpected task auth: %q", request.Header.Get("Authorization"))
			}
			if request.Header.Get("X-ModelScope-Task-Type") != "image_generation" {
				t.Fatalf("unexpected task type header: %q", request.Header.Get("X-ModelScope-Task-Type"))
			}
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"task_status":   "SUCCEED",
				"output_images": []string{server.URL + "/modelscope.png"},
			})
		case "/modelscope.png":
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write(imageBytes)
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	workspacePath := t.TempDir()
	service := NewService(fakeProviderResolver{config: &providercfg.ImageConfig{
		Provider:  "modelscope",
		APIFormat: providercfg.APIFormatModelScopeImageGeneration,
		AuthToken: "modelscope-token",
		BaseURL:   server.URL + "/v1",
		Model:     "Tongyi-MAI/Z-Image-Turbo",
		ProviderOptions: map[string]any{
			"loras": "modelscope-lora",
		},
	}})
	result, payload, err := service.GenerateImage(context.Background(), GenerateInput{
		Prompt:        "A golden cat",
		WorkspacePath: workspacePath,
		FileName:      "modelscope-cat",
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if string(payload) != string(imageBytes) {
		t.Fatalf("payload mismatch")
	}
	if result.Provider != "modelscope" || result.Model != "Tongyi-MAI/Z-Image-Turbo" {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	stored, err := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(result.Path)))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(stored) != string(imageBytes) {
		t.Fatalf("stored file mismatch")
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 14, 8, 0, 0, 0, time.UTC)
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
		t.Fatalf("write test png: %v", err)
	}
}
