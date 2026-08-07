// INPUT: server-fixed owner-main DM business identity and live runtime lease.
// OUTPUT: immutable service Actor and the four safe Channel authorization operations.
// POS: nexus_channel_authorization trusted transport contract.
package contract

import (
	"context"

	authorizationsvc "github.com/nexus-research-lab/nexus/internal/service/channelauthorization"
)

const ServerName = "nexus_channel_authorization"

// ServerContext is injected by the app/runtime builder. No field can be
// supplied or overridden by model tool arguments.
type ServerContext struct {
	OwnerUserID       string
	CurrentAgentID    string
	CurrentSessionKey string
	CurrentRoundID    string
	LeaseSessionKey   string
	LeaseRoundID      string
	ContextKind       string
	ContextID         string
	IsMainAgent       bool
	PrincipalRole     string
	AuthMethod        string
	AuthSessionID     string
	LocalSingleUser   bool
}

func (s ServerContext) Actor() authorizationsvc.Actor {
	return authorizationsvc.Actor{
		OwnerUserID:        s.OwnerUserID,
		AgentID:            s.CurrentAgentID,
		SessionKey:         s.CurrentSessionKey,
		RoundID:            s.CurrentRoundID,
		LeaseSessionKey:    s.LeaseSessionKey,
		LeaseRoundID:       s.LeaseRoundID,
		ContextKind:        s.ContextKind,
		ContextID:          s.ContextID,
		IsMainAgent:        s.IsMainAgent,
		PrincipalRole:      s.PrincipalRole,
		AuthMethod:         s.AuthMethod,
		AuthSessionID:      s.AuthSessionID,
		LocalSingleUser:    s.LocalSingleUser,
		RoundLeaseRequired: true,
	}
}

type Service interface {
	Start(context.Context, authorizationsvc.Actor, authorizationsvc.StartInput) (*authorizationsvc.View, error)
	Status(context.Context, authorizationsvc.Actor, string) (*authorizationsvc.View, error)
	Cancel(context.Context, authorizationsvc.Actor, string) (*authorizationsvc.View, error)
	RequestVerificationCode(context.Context, authorizationsvc.Actor, string) (*authorizationsvc.View, error)
}
