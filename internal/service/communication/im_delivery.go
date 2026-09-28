// INPUT: Host-owned IM delivery records and verified communication identity.
// OUTPUT: Exact-session feedback without transferring previous round authority.
// POS: IM scenario adapter; existing contact and Room messaging remain independent.
package communication

import (
	"context"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
)

type imSessionResolver interface {
	ResolveDeliverySession(context.Context, string) (*protocol.Session, error)
}
type imInputResolver interface {
	IMDeliveryPairing(context.Context, string, string, string) (string, error)
	IMDeliveryInput(context.Context, string, string, string, string, string) (imdelivery.Input, error)
	IMDeliveryContentInput(context.Context, string, string, string, string) (imdelivery.Input, error)
}
type imReplyReceiver interface {
	AcceptIMDeliveryReply(context.Context, imdelivery.Delivery, imdelivery.Reply) error
}
type imScenario struct {
	automationPolicy func(context.Context, imdelivery.Source) (*protocol.RuntimeToolPolicy, error)
	store            *imdelivery.Repository
	sessions         imSessionResolver
	inputs           imInputResolver
	receiver         imReplyReceiver
}

func (s *Service) SetIMDeliveryAdapter(store *imdelivery.Repository, sessions imSessionResolver, inputs imInputResolver, receiver imReplyReceiver) {
	s.im = &imScenario{store: store, sessions: sessions, inputs: inputs, receiver: receiver}
}

func imOrigin(actor Actor) imdelivery.Source {
	sessionKey := actor.SessionKey
	// 公区 key 不是可持久查询的成员 Session；保留原话题的精确执行会话。
	if protocol.ParseSessionKey(sessionKey).Kind == protocol.SessionKeyKindRoom && actor.ConversationID != "" {
		sessionKey = protocol.BuildRoomAgentSessionKey(actor.ConversationID, actor.AgentID, protocol.RoomTypeGroup)
	}
	return imdelivery.Source{Kind: "tool", AgentID: actor.AgentID, SessionKey: sessionKey, RoundID: actor.RoundID, CallID: actor.CallID, RoomID: actor.RoomID, ConversationID: actor.ConversationID}
}
func withIMOrigin(ctx context.Context, actor Actor) context.Context {
	return channels.WithIMDeliverySource(ctx, imOrigin(actor))
}

func (s *Service) SetIMAutomationPolicy(policy func(context.Context, imdelivery.Source) (*protocol.RuntimeToolPolicy, error)) {
	s.im.automationPolicy = policy
}
