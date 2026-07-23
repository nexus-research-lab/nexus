// INPUT: Nexus Responses RuntimeConfig 与显式本地 nxs binary。
// OUTPUT: Nexus→bridge→nxs→/v1/responses 的真实进程级回归。
// POS: 产品配置投影和 runtime 执行边界的 opt-in 集成测试。

package clientopts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

type responsesIntegrationResolver struct {
	config *RuntimeConfig
}

// ResolveRuntimeConfig 返回进程级测试使用的固定模型配置。
func (r responsesIntegrationResolver) ResolveRuntimeConfig(context.Context, string, string) (*RuntimeConfig, error) {
	return r.config, nil
}

// TestNexusResponsesRuntimeProcessIntegration 验证产品配置最终命中真实 nxs 的 Responses provider。
func TestNexusResponsesRuntimeProcessIntegration(t *testing.T) {
	commandPath := strings.TrimSpace(os.Getenv("NEXUS_TEST_NXS_RESPONSES_COMMAND"))
	if commandPath == "" {
		t.Skip("set NEXUS_TEST_NXS_RESPONSES_COMMAND to run the Nexus Responses process integration test")
	}
	t.Setenv("NEXUS_STATE_ROOT", t.TempDir())
	t.Setenv("NEXUS_CONFIG_DIR", "")
	t.Setenv("NEXUS_NXS_COMMAND_PATH", commandPath)

	var (
		mu           sync.Mutex
		requestPaths []string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requestPaths = append(requestPaths, request.URL.Path)
		mu.Unlock()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("解析 Responses 请求: %v", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if payload["store"] != false || payload["model"] != "gpt-test" {
			t.Errorf("Responses payload 不正确: %#v", payload)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		response := map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": "resp_nexus_integration", "model": "gpt-test", "status": "completed",
				"output": []any{map[string]any{
					"type": "message", "id": "msg_nexus_integration", "role": "assistant", "status": "completed",
					"content": []any{map[string]any{"type": "output_text", "text": "nexus-responses-ok"}},
				}},
				"usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
			},
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Errorf("编码 Responses 响应: %v", err)
			return
		}
		_, _ = writer.Write([]byte("data: " + string(encoded) + "\n\n"))
	}))
	defer upstream.Close()

	workspace := t.TempDir()
	options, err := BuildAgentClientOptions(context.Background(), responsesIntegrationResolver{config: &RuntimeConfig{
		Provider:  "azure-responses",
		AuthToken: "test-key",
		BaseURL:   upstream.URL,
		Model:     "gpt-test",
		APIFormat: apiFormatResponses,
	}}, AgentClientOptionsInput{
		WorkspacePath: workspace,
		RuntimeKind:   "nxs",
		Provider:      "azure-responses",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions() error = %v", err)
	}
	if options.Env[nexusAPIProviderEnvName] != "openai" || options.Env[nexusOpenAIProtocolEnvName] != apiFormatResponses {
		t.Fatalf("Responses runtime env = %#v", options.Env)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := agentclient.NewSession(ctx, options)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	defer func() { _ = session.Close(context.Background()) }()
	stream, err := session.Send(ctx, "Reply with exactly: nexus-responses-ok")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	result, err := stream.Result(ctx)
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}
	if result.IsError || !strings.Contains(result.Result, "nexus-responses-ok") {
		t.Fatalf("result = %#v", result)
	}
	mu.Lock()
	paths := append([]string(nil), requestPaths...)
	mu.Unlock()
	if len(paths) != 1 || paths[0] != "/v1/responses" {
		t.Fatalf("Responses 请求路径 = %#v，期望 [/v1/responses]", paths)
	}
}
