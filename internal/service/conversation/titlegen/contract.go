package titlegen

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type providerResolver interface {
	ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error)
}

type sessionService interface {
	GetSession(context.Context, string) (*protocol.Session, error)
	UpdateSessionTitle(context.Context, string, string) (*protocol.Session, error)
}

type roomService interface {
	GetConversationContext(context.Context, string) (*protocol.ConversationContextAggregate, error)
	UpdateConversationTitle(context.Context, string, string, string) (*protocol.ConversationContextAggregate, error)
}

type eventBroadcaster interface {
	BroadcastEvent(context.Context, string, protocol.EventMessage) []error
}

type preferencesService interface {
	Get(context.Context, string) (preferencessvc.Preferences, error)
}
