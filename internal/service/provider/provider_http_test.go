package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestDashScopeImageProviderTestUsesMultimodalPayload(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Fatalf("DashScope 测试路径不正确: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer image-key" {
			t.Fatalf("DashScope 测试鉴权头不正确: %q", request.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("解析 DashScope 测试请求失败: %v", err)
		}
		if body["model"] != "wan2.7-image-pro" {
			t.Fatalf("DashScope 测试模型不正确: %+v", body)
		}
		parameters := body["parameters"].(map[string]any)
		if parameters["n"].(float64) != 1 || parameters["size"] != "1K" || parameters["watermark"] != false {
			t.Fatalf("DashScope 测试参数不正确: %+v", parameters)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"output": map[string]any{
				"finished": true,
				"choices":  []map[string]any{},
			},
		})
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		ProviderKind: ProviderKindImageGeneration,
		Provider:     "dashscope-image",
		PresetKey:    presetCustom,
		APIFormat:    APIFormatDashScopeImageGeneration,
		AuthToken:    "image-key",
		BaseURL:      server.URL,
		ModelsPath:   "",
		Enabled:      true,
		DisplayName:  "DashScope",
	})
	if err != nil {
		t.Fatalf("创建 DashScope 生图 provider 失败: %v", err)
	}
	result, err := service.TestModel(ctx, record.Provider, "wan2.7-image-pro")
	if err != nil {
		t.Fatalf("DashScope 模型测试失败: %v", err)
	}
	if !result.Success || result.Model != "wan2.7-image-pro" {
		t.Fatalf("DashScope 模型测试结果不正确: %+v", result)
	}
	imageConfig, err := service.ResolveImageModelConfig(ctx, record.Provider, "wan2.7-image-pro")
	if err != nil {
		t.Fatalf("DashScope 测试成功后应可解析生图配置: %v", err)
	}
	if imageConfig.APIFormat != APIFormatDashScopeImageGeneration {
		t.Fatalf("DashScope 生图配置未透传 api_format: %+v", imageConfig)
	}
}

func TestModelScopeImageProviderTestUsesAsyncPayload(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/images/generations" {
			t.Fatalf("ModelScope 测试路径不正确: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer image-key" {
			t.Fatalf("ModelScope 测试鉴权头不正确: %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("X-ModelScope-Async-Mode") != "true" {
			t.Fatalf("ModelScope 测试缺少异步请求头: %q", request.Header.Get("X-ModelScope-Async-Mode"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("解析 ModelScope 测试请求失败: %v", err)
		}
		if body["model"] != "Tongyi-MAI/Z-Image-Turbo" || body["prompt"] != "ping" {
			t.Fatalf("ModelScope 测试请求体不正确: %+v", body)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"task_id": "task-test"})
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		ProviderKind: ProviderKindImageGeneration,
		Provider:     "modelscope-image",
		PresetKey:    presetCustom,
		APIFormat:    APIFormatModelScopeImageGeneration,
		AuthToken:    "image-key",
		BaseURL:      server.URL + "/v1",
		ModelsPath:   "",
		Enabled:      true,
		DisplayName:  "ModelScope",
	})
	if err != nil {
		t.Fatalf("创建 ModelScope 生图 provider 失败: %v", err)
	}
	result, err := service.TestModel(ctx, record.Provider, "Tongyi-MAI/Z-Image-Turbo")
	if err != nil {
		t.Fatalf("ModelScope 模型测试失败: %v", err)
	}
	if !result.Success || result.Model != "Tongyi-MAI/Z-Image-Turbo" {
		t.Fatalf("ModelScope 模型测试结果不正确: %+v", result)
	}
	imageConfig, err := service.ResolveImageModelConfig(ctx, record.Provider, "Tongyi-MAI/Z-Image-Turbo")
	if err != nil {
		t.Fatalf("ModelScope 测试成功后应可解析生图配置: %v", err)
	}
	if imageConfig.APIFormat != APIFormatModelScopeImageGeneration {
		t.Fatalf("ModelScope 生图配置未透传 api_format: %+v", imageConfig)
	}
}

func TestDoubaoSeedreamImageProviderUsesArkImagesPayload(t *testing.T) {
	item := providerstore.Entity{
		ProviderKind: ProviderKindImageGeneration,
		Provider:     "doubao",
		PresetKey:    presetDoubao,
		APIFormat:    APIFormatOpenAIImageGeneration,
		BaseURL:      "https://ark.cn-beijing.volces.com/api/v3",
	}
	if got := endpointURL(item, item.APIFormat); got != "https://ark.cn-beijing.volces.com/api/v3/images/generations" {
		t.Fatalf("Doubao Seedream 生图 endpoint 不正确: %s", got)
	}
	payload, err := minimalPayload(item, "doubao-seedream-5-0-260128")
	if err != nil {
		t.Fatalf("生成 Doubao Seedream 测试 payload 失败: %v", err)
	}
	var body map[string]any
	if err = json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("解析 Doubao Seedream 测试 payload 失败: %v", err)
	}
	if body["model"] != "doubao-seedream-5-0-260128" ||
		body["prompt"] != "ping" ||
		body["n"] != float64(1) ||
		body["size"] != "2K" ||
		body["watermark"] != false {
		t.Fatalf("Doubao Seedream 测试 payload 不正确: %+v", body)
	}
}

func TestProviderTestPayloadsForSupportedAPIFormats(t *testing.T) {
	cases := []struct {
		name         string
		apiFormat    string
		expectedPath string
		assertBody   func(t *testing.T, body map[string]any)
	}{
		{
			name:         "chat",
			apiFormat:    APIFormatChatCompletions,
			expectedPath: "/chat/completions",
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if body["model"] != "model-1" || body["max_tokens"] != float64(1) || body["messages"] == nil {
					t.Fatalf("chat payload 不正确: %+v", body)
				}
			},
		},
		{
			name:         "responses",
			apiFormat:    APIFormatResponses,
			expectedPath: "/responses",
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if body["model"] != "model-1" || body["max_output_tokens"] != float64(1) || body["input"] != "ping" {
					t.Fatalf("responses payload 不正确: %+v", body)
				}
			},
		},
		{
			name:         "anthropic",
			apiFormat:    APIFormatAnthropicMessages,
			expectedPath: "/v1/messages",
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if body["model"] != "model-1" || body["max_tokens"] != float64(1) || body["messages"] == nil {
					t.Fatalf("anthropic payload 不正确: %+v", body)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			service, _ := newTestService(t)
			var calledPath string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/models" {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(`{"data":[{"id":"model-1"}]}`))
					return
				}
				calledPath = request.URL.Path
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatalf("解析请求 payload 失败: %v", err)
				}
				tc.assertBody(t, body)
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{}`))
			}))
			defer server.Close()

			record, err := service.Create(ctx, CreateInput{
				Provider:   "provider-" + tc.name,
				PresetKey:  presetCustom,
				APIFormat:  tc.apiFormat,
				AuthToken:  "token-1",
				BaseURL:    server.URL,
				ModelsPath: "/models",
				Enabled:    true,
			})
			if err != nil {
				t.Fatalf("创建 provider 失败: %v", err)
			}
			result, err := service.TestProvider(ctx, record.Provider)
			if err != nil {
				t.Fatalf("TestProvider 返回错误: %v", err)
			}
			if !result.Success {
				t.Fatalf("测试应成功: %+v", result)
			}
			if calledPath != tc.expectedPath {
				t.Fatalf("请求路径不正确: got=%s want=%s", calledPath, tc.expectedPath)
			}
		})
	}
}

func TestProviderTestRedactsSensitiveErrors(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	var mu sync.Mutex
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		if request.URL.Path == "/models" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"data":[{"id":"model-1"}]}`))
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":"Bearer secret-token Authorization x-api-key"}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:   "redact",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatChatCompletions,
		AuthToken:  "secret-token",
		BaseURL:    server.URL,
		ModelsPath: "/models",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	result, err := service.TestProvider(ctx, record.Provider)
	if err != nil {
		t.Fatalf("TestProvider 不应返回 transport 错误: %v", err)
	}
	if result.Success {
		t.Fatalf("测试应失败: %+v", result)
	}
	for _, leaked := range []string{"secret-token", "Authorization", "x-api-key"} {
		if strings.Contains(result.Error, leaked) {
			t.Fatalf("错误信息泄漏敏感内容 %q: %s", leaked, result.Error)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if requestCount != 2 {
		t.Fatalf("Provider 测试应先 /models 再模型请求: got=%d", requestCount)
	}
}
