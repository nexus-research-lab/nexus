package goalobjective

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

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

type fakeRoomLookup struct {
	contexts map[string]*protocol.ConversationContextAggregate
}

func (f fakeRoomLookup) GetConversationContext(_ context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	return f.contexts[conversationID], nil
}
