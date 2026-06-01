package goalobjective

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

func TestRewriteUsesBackgroundPreferenceAndSanitizesObjective(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var receivedSystem string
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			mu.Lock()
			decodeErr = err
			mu.Unlock()
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		messages, _ := payload["messages"].([]any)
		if len(messages) > 0 {
			if first, ok := messages[0].(map[string]any); ok {
				mu.Lock()
				receivedSystem, _ = first["content"].(string)
				mu.Unlock()
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": "\"完成 Goal 对齐并验证关键路径\"",
				},
			}},
		})
	}))
	defer server.Close()

	resolver := &fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider:  "background-provider",
			AuthToken: "token",
			BaseURL:   server.URL + "/v1",
			Model:     "test-model",
			APIFormat: "chat_completions",
		},
	}
	service := NewService(resolver, fakePreferencesService{prefs: preferencessvc.Preferences{
		DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
			Provider: "background-provider",
			Model:    "test-model",
		},
	}})

	got, err := service.Rewrite(context.Background(), Request{
		OwnerUserID: "owner-1",
		Objective:   "把 goal 分支修到和 Codex 差不多",
	})
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if got != "完成 Goal 对齐并验证关键路径" {
		t.Fatalf("Rewrite() = %q", got)
	}
	mu.Lock()
	gotSystem := receivedSystem
	gotDecodeErr := decodeErr
	mu.Unlock()
	if gotDecodeErr != nil {
		t.Fatalf("decode request: %v", gotDecodeErr)
	}
	if gotSystem == "" {
		t.Fatal("missing system prompt")
	}
	for _, want := range []string{"不要缩小", "可验证", "验收条件"} {
		if !strings.Contains(gotSystem, want) {
			t.Fatalf("system prompt = %q, want %q", gotSystem, want)
		}
	}
	if resolver.provider != "background-provider" || resolver.model != "test-model" {
		t.Fatalf("resolver args = %q/%q", resolver.provider, resolver.model)
	}
}

type fakeProviderResolver struct {
	config   *clientopts.RuntimeConfig
	provider string
	model    string
}

func (f *fakeProviderResolver) ResolveLLMConfig(_ context.Context, provider string, model string) (*clientopts.RuntimeConfig, error) {
	f.provider = provider
	f.model = model
	return f.config, nil
}

type fakePreferencesService struct {
	prefs preferencessvc.Preferences
}

func (f fakePreferencesService) Get(context.Context, string) (preferencessvc.Preferences, error) {
	return f.prefs, nil
}
