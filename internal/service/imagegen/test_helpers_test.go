package imagegen

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
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

type fakePreferencesService struct {
	prefs preferencessvc.Preferences
}

func (f fakePreferencesService) Get(_ context.Context, _ string) (preferencessvc.Preferences, error) {
	return f.prefs, nil
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 14, 8, 0, 0, 0, time.UTC)
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
