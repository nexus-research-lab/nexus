// INPUT: Explicit opt-in local SQLite path and exact provider/model targets.
// OUTPUT: Live probe observations using existing credentials, persisted only to an isolated test DB.
// POS: Optional external acceptance; default test runs skip and never access local credentials.
package provider

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveConfiguredModelCapabilities(t *testing.T) {
	path := os.Getenv("NEXUS_PROVIDER_LIVE_DB")
	targets := os.Getenv("NEXUS_PROVIDER_LIVE_TARGETS")
	if path == "" || targets == "" {
		t.Skip("explicit local DB and targets required for live requests")
	}
	databaseURL := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal("open read-only local configuration failed")
	}
	defer db.Close()
	owner := os.Getenv("NEXUS_PROVIDER_LIVE_OWNER")
	if owner == "" {
		owner = "__system__"
	}
	for _, target := range strings.Split(targets, ",") {
		provider, modelID, ok := strings.Cut(strings.TrimSpace(target), "/")
		if !ok || provider == "" || modelID == "" {
			t.Fatal("targets must be exact provider/model pairs")
		}
		t.Run(target, func(t *testing.T) {
			var token, baseURL, format string
			err := db.QueryRow(`SELECT p.auth_token, p.base_url, p.api_format FROM provider p JOIN provider_models m ON m.provider_id=p.id
				WHERE p.owner_user_id=? AND p.provider=? AND m.model_id=? AND p.enabled=1 AND m.enabled=1 AND p.provider_kind='llm'`, owner, provider, modelID).Scan(&token, &baseURL, &format)
			if err != nil {
				t.Fatal("enabled local target not found")
			}
			service, _ := newTestService(t)
			service.SetHTTPClient(&http.Client{Timeout: 30 * time.Second, Transport: http.DefaultTransport})
			ctx := context.Background()
			// Custom routing avoids preset-specific recommendations. Read probe evidence
			// directly below so exact-ID catalog defaults cannot masquerade as live results.
			record, err := service.Create(ctx, CreateInput{Provider: "live-probe", PresetKey: presetCustom, APIFormat: format, AuthToken: token, BaseURL: baseURL, Enabled: true})
			if err != nil {
				t.Fatal("isolated provider setup failed")
			}
			result, err := service.TestModel(ctx, record.Provider, modelID)
			if err != nil {
				t.Fatal("live probe persistence failed")
			}
			if !result.Success {
				t.Fatalf("live connectivity failed: %s", result.Error)
			}
			current, err := service.Get(ctx, record.Provider)
			if err != nil || len(current.Models) != 1 {
				t.Fatal("probe result missing")
			}
			item, err := service.requireProvider(ctx, record.Provider)
			if err != nil {
				t.Fatal("probe provider missing")
			}
			stored, err := service.getModelByID(ctx, item.ID, modelID)
			if err != nil || stored == nil {
				t.Fatal("stored model missing")
			}
			probe := currentModelProbe(*item, *stored)
			if probe == nil {
				t.Fatal("configuration-bound live evidence missing")
			}
			caps := probe.Capabilities
			state := func(value *bool) string {
				if value == nil {
					return "unknown"
				}
				if *value {
					return "supported"
				}
				return "unsupported"
			}
			t.Logf("text=%s vision=%s tools=%s reasoning=%s image_output=%s image_editing=%s embedding=%s", state(caps.TextOutput), state(caps.Vision), state(caps.ToolCalling), state(caps.Reasoning), state(caps.ImageOutput), state(caps.ImageEditing), state(caps.Embedding))
			if caps.TextOutput == nil || !*caps.TextOutput {
				t.Error("no text capability confirmed")
			}
		})
	}
}
