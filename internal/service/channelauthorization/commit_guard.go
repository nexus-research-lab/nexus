// INPUT: exact Channel login identity at the credential-commit boundary.
// OUTPUT: a fresh human/owner-main validation leased through persistence and reload.
// POS: fail-closed bridge from Channel login completion into conversational authorization.
package channelauthorization

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	channelssvc "github.com/nexus-research-lab/nexus/internal/service/channels"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	authorizationstore "github.com/nexus-research-lab/nexus/internal/storage/channelauthorization"
)

// AcquireChannelLoginAuthorizationCommit verifies the exact durable flow and
// returns the service lifecycle write lease. It excludes cancellation, expiry,
// monitor reconciliation, and Close until the credential commit or rollback
// and runtime publication have all finished.
func (s *Service) AcquireChannelLoginAuthorizationCommit(
	ctx context.Context,
	request channelssvc.ChannelLoginAuthorizationCommit,
) (release func(), err error) {
	endOperation, err := s.beginCommitOperation()
	if err != nil {
		return nil, err
	}
	keepLease := false
	defer func() {
		if !keepLease {
			endOperation()
		}
	}()
	if err = s.Initialize(ctx); err != nil {
		return nil, err
	}
	request.OwnerUserID = strings.TrimSpace(request.OwnerUserID)
	request.ChannelType = strings.TrimSpace(request.ChannelType)
	request.LoginID = strings.TrimSpace(request.LoginID)
	request.AuthorizationBinding = strings.TrimSpace(request.AuthorizationBinding)
	if request.OwnerUserID == "" ||
		request.ChannelType == "" ||
		request.LoginID == "" ||
		request.AuthorizationBinding == "" ||
		request.StartControlVersion <= 0 {
		return nil, errors.New("Channel authorization commit binding is incomplete")
	}
	flow, err := s.repository.GetActiveByGeneration(
		ctx,
		request.OwnerUserID,
		request.ChannelType,
		request.AuthorizationBinding,
		s.processGeneration,
	)
	if err != nil {
		return nil, err
	}
	if flow.StartControlVersion != request.StartControlVersion ||
		!authorizationstore.IsActiveStatus(flow.Status) ||
		!s.now().Before(flow.ExpiresAt) {
		return nil, errors.New("Channel authorization commit lease is stale")
	}
	if strings.TrimSpace(flow.RuntimeRefEncrypted) == "" {
		return nil, errors.New("Channel authorization commit flow is missing its login reference")
	}
	ref, refErr := s.decryptRuntimeReference(flow.RuntimeRefEncrypted)
	if refErr != nil ||
		strings.TrimSpace(ref.LoginID) != request.LoginID ||
		strings.TrimSpace(ref.ChannelType) != request.ChannelType {
		return nil, errors.New("Channel authorization commit login does not match the durable flow")
	}
	if s.humanVerifier == nil {
		return nil, errors.New("Channel authorization human verifier is unavailable")
	}
	principal, releaseHuman, err := s.humanVerifier.AcquireBoundInteractiveHumanLease(
		ctx,
		flow.PrincipalUserID,
		flow.PrincipalAuthMethod,
		flow.PrincipalAuthSessionID,
	)
	if err != nil {
		return nil, err
	}
	keepHumanLease := false
	defer func() {
		if !keepHumanLease && releaseHuman != nil {
			releaseHuman()
		}
	}()
	if err = requireCommitPrincipal(flow.Binding, principal); err != nil {
		return nil, err
	}
	actor := Actor{
		OwnerUserID:        flow.OwnerUserID,
		AgentID:            flow.AgentID,
		SessionKey:         flow.BusinessSessionKey,
		RoundID:            flow.RootRoundID,
		LeaseSessionKey:    flow.RuntimeLeaseSessionKey,
		LeaseRoundID:       flow.RuntimeLeaseRoundID,
		IsMainAgent:        true,
		ContextKind:        configurationsvc.ContextKindAgent,
		ContextID:          flow.AgentID,
		PrincipalRole:      flow.PrincipalRole,
		AuthMethod:         flow.PrincipalAuthMethod,
		AuthSessionID:      flow.PrincipalAuthSessionID,
		LocalSingleUser:    flow.PrincipalAuthMethod == authctx.AuthMethodLocal,
		RoundLeaseRequired: false,
	}
	if s.authority == nil {
		return nil, errors.New("Channel authorization authority verifier is unavailable")
	}
	inspection, err := s.authority.Inspect(
		authctx.WithPrincipal(ctx, principal),
		actor,
		[]string{configurationsvc.DomainChannels},
		false,
	)
	if err != nil {
		return nil, err
	}
	if inspection == nil ||
		inspection.Authority != configurationsvc.AuthorityOwnerMain ||
		inspection.Context.Kind != configurationsvc.ScopeKindOwner {
		return nil, errors.New("Channel authorization owner-main authority is no longer active")
	}
	keepLease = true
	keepHumanLease = true
	var releaseOnce sync.Once
	return func() {
		releaseOnce.Do(func() {
			releaseHuman()
			endOperation()
		})
	}, nil
}

func requireCommitPrincipal(
	binding authorizationstore.Binding,
	principal *authctx.Principal,
) error {
	if principal == nil {
		return errors.New("Channel authorization human principal is unavailable")
	}
	sessionID := ""
	if principal.SessionID != nil {
		sessionID = strings.TrimSpace(*principal.SessionID)
	}
	if strings.TrimSpace(principal.UserID) != strings.TrimSpace(binding.PrincipalUserID) ||
		strings.TrimSpace(principal.Role) != strings.TrimSpace(binding.PrincipalRole) ||
		strings.TrimSpace(principal.AuthMethod) != strings.TrimSpace(binding.PrincipalAuthMethod) ||
		sessionID != strings.TrimSpace(binding.PrincipalAuthSessionID) {
		return errors.New("Channel authorization original human principal/session changed")
	}
	return nil
}
