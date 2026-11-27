// INPUT: 并发 owner 更新、陈旧 version、WebSearch credential 与回滚快照。
// OUTPUT: Preferences 串行 RMW、CAS、条件回滚和唯一临时文件行为证明。
// POS: Preferences 文件真相源 P0 一致性回归测试。
package preferences

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestServiceClearsWebSearchAPIKeyWhenProviderChanges(t *testing.T) {
	root := t.TempDir()
	service := NewService(config.Config{WorkspacePath: filepath.Join(root, "workspace")})
	apiKey := "brave-key"
	if _, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch:       &WebSearchSettings{Enabled: true, Provider: "brave"},
		WebSearchAPIKey: &apiKey,
	}); err != nil {
		t.Fatalf("写入 Brave 配置失败: %v", err)
	}

	prefs, err := service.Update(context.Background(), "user/1", UpdateRequest{
		WebSearch: &WebSearchSettings{Provider: "tavily"},
	})
	if err != nil {
		t.Fatalf("切换 Tavily 失败: %v", err)
	}
	if prefs.WebSearch.APIKeyConfigured || prefs.WebSearchAPIKey() != "" {
		t.Fatalf("切换 provider 后不应复用旧 API key: %+v", prefs.WebSearch)
	}
	keyPath := testUserSettingsPath(root, "user/1", "web-search-api-key")
	if _, err := os.Stat(keyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("切换 provider 后应删除旧 API key: %v", err)
	}
}

func TestServiceRejectsIncompleteWebSearchSettings(t *testing.T) {
	service := NewService(config.Config{WorkspacePath: t.TempDir()})
	tests := []WebSearchSettings{
		{Enabled: true, Provider: "brave"},
		{Enabled: true, Provider: "searxng"},
		{Enabled: true, Provider: "unsupported"},
	}
	for _, settings := range tests {
		if _, err := service.Update(context.Background(), "user/1", UpdateRequest{WebSearch: &settings}); err == nil {
			t.Fatalf("无效 WebSearch 配置应被拒绝: %+v", settings)
		}
	}
}

func testUserSettingsPath(root string, ownerUserID string, fileName string) string {
	return filepath.Join(
		root,
		"workspace",
		appfs.UserPathSegment(ownerUserID),
		"workspace",
		".settings",
		fileName,
	)
}
