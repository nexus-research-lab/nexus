package preferences

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDefaultPreferencesAskByDefault(t *testing.T) {
	prefs := DefaultPreferences()
	if prefs.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("默认权限应为询问模式: %+v", prefs.DefaultAgentOptions)
	}
	if len(prefs.DefaultAgentOptions.AllowedTools) != 0 {
		t.Fatalf("默认不应预授权工具: %+v", prefs.DefaultAgentOptions.AllowedTools)
	}
	if prefs.AgentRuntimeKind != "claude" {
		t.Fatalf("默认 runtime 应为 claude: %+v", prefs)
	}

	normalized := normalizePreferences(Preferences{})
	if normalized.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("空偏好归一化后应为询问模式: %+v", normalized.DefaultAgentOptions)
	}
	if normalized.AgentRuntimeKind != "claude" {
		t.Fatalf("空偏好归一化后 runtime 应为 claude: %+v", normalized)
	}
}

func TestServiceUpdatePersistsUserPreferences(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyQueue),
		AgentRuntimeKind:          stringPointer("nxs"),
		DefaultAgentOptions: &protocol.Options{
			PermissionMode: "default",
			Provider:       "glm-coding-plan",
			Model:          "glm-5.1",
			AllowedTools:   []string{"Read", "Read", "Write"},
		},
		DefaultImageModelSelection: &ModelSelection{
			Provider: "image-provider",
			Model:    "image-model",
		},
		DefaultBackgroundModelSelection: &ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	})
	if err != nil {
		t.Fatalf("更新偏好失败: %v", err)
	}
	if prefs.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("消息行为未持久化: %+v", prefs)
	}
	if prefs.AgentRuntimeKind != "nxs" {
		t.Fatalf("runtime 偏好未持久化: %+v", prefs)
	}
	if prefs.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("权限模式未持久化: %+v", prefs.DefaultAgentOptions)
	}
	if len(prefs.DefaultAgentOptions.AllowedTools) != 2 {
		t.Fatalf("工具列表应去重: %+v", prefs.DefaultAgentOptions.AllowedTools)
	}
	if prefs.DefaultAgentOptions.Provider != "glm-coding-plan" || prefs.DefaultAgentOptions.Model != "glm-5.1" {
		t.Fatalf("默认 Agent 模型未持久化: %+v", prefs.DefaultAgentOptions)
	}
	if prefs.DefaultImageModelSelection.Provider != "image-provider" || prefs.DefaultImageModelSelection.Model != "image-model" {
		t.Fatalf("默认生图模型未持久化: %+v", prefs.DefaultImageModelSelection)
	}
	if prefs.DefaultBackgroundModelSelection.Provider != "background-provider" || prefs.DefaultBackgroundModelSelection.Model != "background-model" {
		t.Fatalf("后台任务模型未持久化: %+v", prefs.DefaultBackgroundModelSelection)
	}

	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取偏好失败: %v", err)
	}
	if loaded.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyQueue ||
		loaded.AgentRuntimeKind != "nxs" ||
		loaded.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("读取结果不正确: %+v", loaded)
	}
	if loaded.DefaultImageModelSelection.Model != "image-model" || loaded.DefaultBackgroundModelSelection.Model != "background-model" {
		t.Fatalf("读取默认模型选择不正确: %+v", loaded)
	}
	if _, statErr := os.Stat(filepath.Join(root, "workspace", "user_1", ".settings", "preferences.json")); statErr != nil {
		t.Fatalf("偏好文件未写入安全路径: %v", statErr)
	}
}

func TestServiceUpdateNormalizesRuntimeKindAlias(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		AgentRuntimeKind: stringPointer("NXS"),
	})
	if err != nil {
		t.Fatalf("更新 runtime 偏好失败: %v", err)
	}
	if prefs.AgentRuntimeKind != "nxs" {
		t.Fatalf("runtime 别名未归一化: %+v", prefs)
	}
}

func TestServiceUpdatePersistsInterruptDefaultDeliveryPolicy(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyInterrupt),
	})
	if err != nil {
		t.Fatalf("更新偏好失败: %v", err)
	}
	if prefs.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyInterrupt {
		t.Fatalf("打断默认行为未持久化: %+v", prefs)
	}
}

func policyPointer(value protocol.ChatDeliveryPolicy) *protocol.ChatDeliveryPolicy {
	return &value
}

func stringPointer(value string) *string {
	return &value
}
