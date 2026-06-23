package titlegen

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

var errTestNotFound = errors.New("not found")

type fakePreferencesService struct {
	prefs preferencessvc.Preferences
}

func (f fakePreferencesService) Get(_ context.Context, _ string) (preferencessvc.Preferences, error) {
	return f.prefs, nil
}

type fakeProviderResolver struct {
	config   *clientopts.RuntimeConfig
	provider string
	model    string
}

func (f *fakeProviderResolver) ResolveLLMConfig(
	_ context.Context,
	provider string,
	model string,
) (*clientopts.RuntimeConfig, error) {
	f.provider = provider
	f.model = model
	return f.config, nil
}

type fakeSessionService struct {
	sessions map[string]*protocol.Session
}

func (f *fakeSessionService) GetSession(_ context.Context, sessionKey string) (*protocol.Session, error) {
	item := f.sessions[sessionKey]
	if item == nil {
		return nil, errTestNotFound
	}
	value := *item
	return &value, nil
}

func (f *fakeSessionService) UpdateSessionTitle(_ context.Context, sessionKey string, title string) (*protocol.Session, error) {
	item := f.sessions[sessionKey]
	if item == nil {
		return nil, errTestNotFound
	}
	item.Title = title
	value := *item
	return &value, nil
}

type fakeRoomService struct {
	contexts map[string]*protocol.ConversationContextAggregate
}

func (f *fakeRoomService) GetConversationContext(_ context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	item := f.contexts[conversationID]
	if item == nil {
		return nil, errTestNotFound
	}
	value := *item
	return &value, nil
}

func (f *fakeRoomService) UpdateConversationTitle(
	_ context.Context,
	_ string,
	conversationID string,
	title string,
) (*protocol.ConversationContextAggregate, error) {
	item := f.contexts[conversationID]
	if item == nil {
		return nil, errTestNotFound
	}
	item.Conversation.Title = title
	value := *item
	return &value, nil
}

type fakeEventBroadcaster struct {
	events []protocol.EventMessage
}

func (f *fakeEventBroadcaster) BroadcastEvent(_ context.Context, _ string, event protocol.EventMessage) []error {
	f.events = append(f.events, event)
	return nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}
