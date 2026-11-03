// INPUT: Built Web UI, isolated real server/database, real nxs and a local scripted model provider.
// OUTPUT: Browser authentication, WebSocket chat and persisted history across reload.
// POS: Opt-in/nightly integration smoke; no user state, remote model credentials or API route mocks.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	providersvc "github.com/nexus-research-lab/nexus/internal/service/provider"
)

type browserSmokeFactory struct{ binary string }

func (f browserSmokeFactory) New(options bridge.Options) runtimectx.Client {
	options.CLIPath = f.binary
	return runtimectx.NewAgentClient(options)
}

func TestBrowserSmoke(t *testing.T) {
	if os.Getenv("NEXUS_BROWSER_SMOKE") != "1" {
		t.Skip("set NEXUS_BROWSER_SMOKE=1 after building web and installing Chromium")
	}
	binary := os.Getenv("NEXUS_SMOKE_NXS_BINARY")
	if binary == "" {
		t.Fatal("NEXUS_SMOKE_NXS_BINARY must point to the paired nxs runtime")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("runtime path must be absolute")
	}
	webRoot, err := filepath.Abs("../../../web")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(webRoot, "dist", "app.html")); err != nil {
		t.Fatal("build web before smoke: ", err)
	}
	pnpm, err := exec.LookPath("pnpm")
	if err != nil {
		t.Fatal(err)
	}
	browserCache := os.Getenv("PLAYWRIGHT_BROWSERS_PATH")
	if browserCache == "" {
		cacheRoot, err := os.UserCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		browserCache = filepath.Join(cacheRoot, "ms-playwright")
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []map[string]any{
			{"type": "message_start", "message": map[string]any{"id": "smoke-reply", "model": "claude-smoke", "role": "assistant", "content": []any{}, "usage": map[string]int{"input_tokens": 1}}},
			{"type": "content_block_start", "index": 0, "content_block": map[string]string{"type": "text", "text": ""}},
			{"type": "content_block_delta", "index": 0, "delta": map[string]string{"type": "text_delta", "text": "Nexus real backend smoke reply"}},
			{"type": "content_block_stop", "index": 0},
			{"type": "message_delta", "delta": map[string]string{"stop_reason": "end_turn"}, "usage": map[string]int{"output_tokens": 8}},
			{"type": "message_stop"},
		} {
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data)
		}
	}))
	defer provider.Close()
	cfg := handlertest.NewConfig(t)
	cfg.AppMode = "desktop"
	cfg.DesktopSessionToken = "smoke-local-token"
	cfg.WebDistDir = filepath.Join(webRoot, "dist")
	cfg.MultiplayerDisabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	handlertest.CloseServer(t, server)
	ctx := authctx.WithPrincipal(t.Context(), &authctx.Principal{UserID: authctx.SystemUserID, Username: "local", Role: "admin", AuthMethod: "local"})
	record, err := server.services.Provider.Create(ctx, providersvc.CreateInput{Provider: "smoke-provider", PresetKey: "custom", APIFormat: "anthropic_messages", AuthToken: "local-only", BaseURL: provider.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	_, err = server.services.Provider.UpdateModel(ctx, record.Provider, "claude-smoke", providersvc.UpdateModelInput{Enabled: true, IsDefault: true, CapabilitiesOverride: providersvc.ModelCapabilities{TextOutput: &yes, ToolCalling: &yes}})
	if err != nil {
		t.Fatal(err)
	}
	manager := runtimectx.NewManagerWithFactory(browserSmokeFactory{binary: binary})
	defer manager.Close(context.Background())
	server.services.DM.Runtime = manager
	server.services.RoomRealtime.Runtime = manager
	httpServer := httptest.NewServer(server.Router())
	defer httpServer.Close()
	command := exec.CommandContext(t.Context(), pnpm, "exec", "playwright", "test", "--config", "playwright.smoke.config.ts")
	command.Dir = webRoot
	command.Env = append(os.Environ(), "NEXUS_SMOKE_BASE_URL="+httpServer.URL, "NEXUS_SMOKE_TOKEN="+cfg.DesktopSessionToken, "PLAYWRIGHT_BROWSERS_PATH="+browserCache)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}
