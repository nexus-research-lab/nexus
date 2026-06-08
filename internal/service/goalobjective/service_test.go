package goalobjective

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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

func TestRewritePrefersAgentConversationModelOverBackgroundPreference(t *testing.T) {
	t.Parallel()

	server := newRewriteResponseServer(t, "\"完成 Goal 对齐\"")
	resolver := &fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider:  "conversation-provider",
			AuthToken: "token",
			BaseURL:   server.URL + "/v1",
			Model:     "conversation-model",
			APIFormat: "chat_completions",
		},
	}
	service := NewService(resolver, fakePreferencesService{prefs: preferencessvc.Preferences{
		DefaultAgentOptions: protocol.Options{
			Provider: "default-agent-provider",
			Model:    "default-agent-model",
		},
		DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	}})
	service.SetConversationResolvers(
		fakeAgentLookup{agents: map[string]*protocol.Agent{
			"agent-dev": {
				AgentID: "agent-dev",
				Options: protocol.Options{
					Provider: "conversation-provider",
					Model:    "conversation-model",
				},
			},
		}},
		nil,
	)

	if _, err := service.Rewrite(context.Background(), Request{
		OwnerUserID: "owner-1",
		SessionKey:  "agent:agent-dev:ws:dm:chat",
		Objective:   "把 Goal 模式检查清楚",
	}); err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if resolver.provider != "conversation-provider" || resolver.model != "conversation-model" {
		t.Fatalf("resolver args = %q/%q, want conversation model", resolver.provider, resolver.model)
	}
}

func TestRewriteUsesDefaultAgentModelForConversationAgentWithoutExplicitModel(t *testing.T) {
	t.Parallel()

	server := newRewriteResponseServer(t, "\"完成默认模型检查\"")
	resolver := &fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider:  "default-agent-provider",
			AuthToken: "token",
			BaseURL:   server.URL + "/v1",
			Model:     "default-agent-model",
			APIFormat: "chat_completions",
		},
	}
	service := NewService(resolver, fakePreferencesService{prefs: preferencessvc.Preferences{
		DefaultAgentOptions: protocol.Options{
			Provider: "default-agent-provider",
			Model:    "default-agent-model",
		},
		DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	}})
	service.SetConversationResolvers(
		fakeAgentLookup{agents: map[string]*protocol.Agent{
			"agent-dev": {AgentID: "agent-dev"},
		}},
		nil,
	)

	if _, err := service.Rewrite(context.Background(), Request{
		OwnerUserID: "owner-1",
		SessionKey:  "agent:agent-dev:ws:dm:chat",
		Objective:   "把 Goal 模式检查清楚",
	}); err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if resolver.provider != "default-agent-provider" || resolver.model != "default-agent-model" {
		t.Fatalf("resolver args = %q/%q, want default agent model", resolver.provider, resolver.model)
	}
}

func TestRewriteFallsBackToBackgroundWhenConversationModelIncomplete(t *testing.T) {
	t.Parallel()

	server := newRewriteResponseServer(t, "\"完成后台模型回退检查\"")
	resolver := &fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider:  "background-provider",
			AuthToken: "token",
			BaseURL:   server.URL + "/v1",
			Model:     "background-model",
			APIFormat: "chat_completions",
		},
	}
	service := NewService(resolver, fakePreferencesService{prefs: preferencessvc.Preferences{
		DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	}})
	service.SetConversationResolvers(
		fakeAgentLookup{agents: map[string]*protocol.Agent{
			"agent-dev": {
				AgentID: "agent-dev",
				Options: protocol.Options{
					Provider: "conversation-provider",
				},
			},
		}},
		nil,
	)

	if _, err := service.Rewrite(context.Background(), Request{
		OwnerUserID: "owner-1",
		SessionKey:  "agent:agent-dev:ws:dm:chat",
		Objective:   "把 Goal 模式检查清楚",
	}); err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if resolver.provider != "background-provider" || resolver.model != "background-model" {
		t.Fatalf("resolver args = %q/%q, want background model fallback", resolver.provider, resolver.model)
	}
}

func TestRewriteUsesRoomGoalTargetAgentModel(t *testing.T) {
	t.Parallel()

	server := newRewriteResponseServer(t, "\"完成 Room Goal 检查\"")
	resolver := &fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider:  "host-provider",
			AuthToken: "token",
			BaseURL:   server.URL + "/v1",
			Model:     "host-model",
			APIFormat: "chat_completions",
		},
	}
	service := NewService(resolver, fakePreferencesService{prefs: preferencessvc.Preferences{
		DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	}})
	service.SetConversationResolvers(nil, fakeRoomLookup{contexts: map[string]*protocol.ConversationContextAggregate{
		"conversation-1": {
			Room: protocol.RoomRecord{
				HostAgentID:          "agent-host",
				HostAutoReplyEnabled: true,
			},
			MemberAgents: []protocol.Agent{
				{AgentID: "agent-host", Options: protocol.Options{Provider: "host-provider", Model: "host-model"}},
				{AgentID: "agent-peer", Options: protocol.Options{Provider: "peer-provider", Model: "peer-model"}},
			},
		},
	}})

	if _, err := service.Rewrite(context.Background(), Request{
		OwnerUserID: "owner-1",
		SessionKey:  "room:group:conversation-1",
		Objective:   "把 Room Goal 模式检查清楚",
	}); err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if resolver.provider != "host-provider" || resolver.model != "host-model" {
		t.Fatalf("resolver args = %q/%q, want room host model", resolver.provider, resolver.model)
	}
}

func newRewriteResponseServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": content,
				},
			}},
		})
	}))
	t.Cleanup(server.Close)
	return server
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

type fakeAgentLookup struct {
	agents map[string]*protocol.Agent
}

func (f fakeAgentLookup) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	return f.agents[agentID], nil
}

type fakeRoomLookup struct {
	contexts map[string]*protocol.ConversationContextAggregate
}

func (f fakeRoomLookup) GetConversationContext(_ context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	return f.contexts[conversationID], nil
}
