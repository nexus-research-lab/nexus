// INPUT: configuration Preferences merge patch、资源 state_version 与热同步失败。
// OUTPUT: 锁内最新值 merge、Preferences CAS 和条件回滚行为证明。
// POS: 对话配置不能用 plan 前快照覆盖 UI 写入的 P0 回归测试。
package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

func TestUpdatePreferencesMergesInsideLatestVersionBoundary(t *testing.T) {
	preferences := preferencessvc.NewService(config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	})
	service := &Service{prefs: preferences}
	const ownerID = "owner-configuration-preferences"

	diagnosticsEnabled := true
	uiValue, err := preferences.Update(
		context.Background(),
		ownerID,
		preferencessvc.UpdateRequest{AgentSDKDiagnosticsEnabled: &diagnosticsEnabled},
	)
	if err != nil {
		t.Fatal(err)
	}
	rawInput := json.RawMessage(`{"chat_default_delivery_policy":"interrupt"}`)
	updated, err := service.updatePreferences(
		context.Background(),
		Actor{OwnerUserID: ownerID},
		preferencessvc.UpdateRequest{
			ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyInterrupt),
		},
		rawInput,
		uiValue.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.AgentSDKDiagnosticsEnabled ||
		updated.ChatDefaultDeliveryPolicy != protocol.ChatDeliveryPolicyInterrupt {
		t.Fatalf("对话 merge 覆盖了 UI 字段: %+v", updated)
	}
	if updated.Version != uiValue.Version+1 {
		t.Fatalf("configuration Preferences version = %d, want %d", updated.Version, uiValue.Version+1)
	}

	_, err = service.updatePreferences(
		context.Background(),
		Actor{OwnerUserID: ownerID},
		preferencessvc.UpdateRequest{
			ChatDefaultDeliveryPolicy: policyPointer(protocol.ChatDeliveryPolicyQueue),
		},
		json.RawMessage(`{"chat_default_delivery_policy":"queue"}`),
		uiValue.Version,
	)
	if !errors.Is(err, preferencessvc.ErrVersionConflict) {
		t.Fatalf("陈旧 configuration CAS error = %v", err)
	}
}

func TestUpdatePreferencesRestoresOnlyWrittenWebSearchVersion(t *testing.T) {
	preferences := preferencessvc.NewService(config.Config{
		WorkspacePath: filepath.Join(t.TempDir(), "workspace"),
	})
	service := &Service{prefs: preferences}
	const ownerID = "owner-configuration-rollback"

	initial, err := preferences.Get(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.updatePreferences(
		context.Background(),
		Actor{OwnerUserID: ownerID},
		preferencessvc.UpdateRequest{
			WebSearch: &preferencessvc.WebSearchSettings{DefaultCount: 9},
		},
		json.RawMessage(`{"web_search":{"default_count":9}}`),
		initial.Version,
	)
	if err == nil {
		t.Fatal("缺少 runtime 热同步依赖时应失败并回滚")
	}
	stored, readErr := preferences.Get(context.Background(), ownerID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if stored.WebSearch.DefaultCount != initial.WebSearch.DefaultCount {
		t.Fatalf("WebSearch 热同步失败后未恢复旧值: %+v", stored.WebSearch)
	}
	if stored.Version != initial.Version+2 {
		t.Fatalf("写入和条件回滚应各推进一次 version: got=%d want=%d", stored.Version, initial.Version+2)
	}
}

func policyPointer(value protocol.ChatDeliveryPolicy) *protocol.ChatDeliveryPolicy {
	return &value
}
