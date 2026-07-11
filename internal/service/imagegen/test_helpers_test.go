package imagegen

import (
	"context"
	"os"
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

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
		t.Fatalf("write test png: %v", err)
	}
}
