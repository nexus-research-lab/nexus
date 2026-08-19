package host

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestConfigurationCLIRoutesRuntimeCallsThroughBroker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get(protocol.NexusConfigCapabilityHeader) != "runtime-token" {
			t.Fatalf("capability header = %q", request.Header.Get(protocol.NexusConfigCapabilityHeader))
		}
		var command runtimeConfigurationCommand
		if err := json.NewDecoder(request.Body).Decode(&command); err != nil {
			t.Fatal(err)
		}
		if command.Action != "inspect" || !command.Verify {
			t.Fatalf("broker command = %+v", command)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"authority": "agent_self",
				"context":   map[string]any{"kind": "agent", "id": "agent-a"},
				"domains":   map[string]any{},
			},
		})
	}))
	defer server.Close()
	t.Setenv(protocol.NexusConfigBrokerURLEnvName, server.URL)
	t.Setenv(protocol.NexusConfigCapabilityTokenEnvName, "runtime-token")

	command, err := NewConfiguration(newCLITestConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	command.SetArgs([]string{"--json", "inspect", "--verify"})
	stdout, stderr, err := captureCLIStreams(t, command)
	if err != nil {
		t.Fatalf("runtime inspect err=%v stderr=%s", err, stderr)
	}
	var result map[string]any
	if err = json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	inspection, _ := result["inspection"].(map[string]any)
	if inspection["authority"] != "agent_self" {
		t.Fatalf("inspect result = %#v", result)
	}
}

func TestConfigurationCLIInspectsAndAppliesPreferences(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	inspect, err := NewConfiguration(cfg)
	if err != nil {
		t.Fatalf("创建 nexuscfg 失败: %v", err)
	}
	inspect.SetArgs([]string{"--json", "inspect", "--domain", "preferences"})
	stdout, stderr, err := captureCLIStreams(t, inspect)
	if err != nil {
		t.Fatalf("nexuscfg inspect 失败: %v, stderr=%s", err, stderr)
	}
	var inspected map[string]any
	if err = json.Unmarshal([]byte(stdout), &inspected); err != nil {
		t.Fatalf("解析 inspect 输出失败: %v, stdout=%s", err, stdout)
	}
	if inspected["success"] != true || inspected["inspection"] == nil {
		t.Fatalf("inspect 输出不完整: %#v", inspected)
	}

	apply, err := NewConfiguration(cfg)
	if err != nil {
		t.Fatalf("创建 nexuscfg 失败: %v", err)
	}
	apply.SetArgs([]string{
		"--json", "apply",
		"--domain", "preferences",
		"--operation", "update",
		"--input", `{"chat_default_delivery_policy":"reject"}`,
	})
	stdout, stderr, err = captureCLIStreams(t, apply)
	if err != nil {
		t.Fatalf("nexuscfg apply 失败: %v, stderr=%s", err, stderr)
	}
	var applied map[string]any
	if err = json.Unmarshal([]byte(stdout), &applied); err != nil {
		t.Fatalf("解析 apply 输出失败: %v, stdout=%s", err, stdout)
	}
	if applied["success"] != true || applied["plan"] == nil || applied["result"] == nil {
		t.Fatalf("apply 输出不完整: %#v", applied)
	}
}

func TestConfigurationCLIRequiresExplicitConfirmation(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)
	command, err := NewConfiguration(cfg)
	if err != nil {
		t.Fatalf("创建 nexuscfg 失败: %v", err)
	}
	command.SetArgs([]string{
		"--json", "apply",
		"--domain", "preferences",
		"--operation", "update",
		"--input", `{"agent_runtime_kind":"nxs"}`,
	})
	_, _, err = captureCLIStreams(t, command)
	if err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("高风险配置应要求 --confirm，err=%v", err)
	}
}

func TestConfigurationCLIRejectsAgentSecretInput(t *testing.T) {
	t.Setenv(nexusctlUserIDEnvName, authctx.SystemUserID)
	t.Setenv(nexusRuntimeScopeModeEnvName, runtimeScopeModeSingleUser)
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)
	command, err := NewConfiguration(cfg)
	if err != nil {
		t.Fatalf("创建 nexuscfg 失败: %v", err)
	}
	command.SetArgs([]string{
		"--json", "apply",
		"--domain", "providers",
		"--operation", "create",
		"--input", `{
			"provider":"test-provider",
			"preset_key":"custom",
			"api_format":"responses",
			"display_name":"Test Provider",
			"auth_token":{"$secret":"provider.auth_token"},
			"base_url":"https://provider.example.com/v1",
			"models_path":"/models"
		}`,
		"--secrets-stdin",
	})
	command.SetIn(strings.NewReader(`{"provider.auth_token":"must-not-be-read"}`))
	_, _, err = captureCLIStreams(t, command)
	if err == nil || !strings.Contains(err.Error(), "Agent runtime 不可用") {
		t.Fatalf("Agent runtime 不应允许输入配置秘密，err=%v", err)
	}
}
