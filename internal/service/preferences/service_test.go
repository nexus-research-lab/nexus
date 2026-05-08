package preferences

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceUpdatePersistsUserPreferences(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyQueue),
		DefaultAgentOptions: &protocol.Options{
			PermissionMode: "default",
			AllowedTools:   []string{"Read", "Read", "Write"},
		},
	})
	if err != nil {
		t.Fatalf("更新偏好失败: %v", err)
	}
	if prefs.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyQueue {
		t.Fatalf("消息行为未持久化: %+v", prefs)
	}
	if prefs.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("权限模式未持久化: %+v", prefs.DefaultAgentOptions)
	}
	if len(prefs.DefaultAgentOptions.AllowedTools) != 2 {
		t.Fatalf("工具列表应去重: %+v", prefs.DefaultAgentOptions.AllowedTools)
	}

	loaded, err := service.Get(context.Background(), "user/1")
	if err != nil {
		t.Fatalf("读取偏好失败: %v", err)
	}
	if loaded.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyQueue || loaded.DefaultAgentOptions.PermissionMode != "default" {
		t.Fatalf("读取结果不正确: %+v", loaded)
	}
	if _, statErr := os.Stat(filepath.Join(root, "workspace", "user_1", ".settings", "preferences.json")); statErr != nil {
		t.Fatalf("偏好文件未写入安全路径: %v", statErr)
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
