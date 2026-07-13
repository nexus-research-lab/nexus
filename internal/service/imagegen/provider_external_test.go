package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

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
