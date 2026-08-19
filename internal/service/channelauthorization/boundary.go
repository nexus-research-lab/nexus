// INPUT: server-fixed configuration Actor and a stored flow binding.
// OUTPUT: fresh owner-main DM authorization plus exact cross-scope comparison.
// POS: every conversational Channel authorization operation's dynamic boundary.
package channelauthorization

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	authorizationstore "github.com/nexus-research-lab/nexus/internal/storage/channelauthorization"
)

func (s *Service) authorize(
	ctx context.Context,
	actor Actor,
) (authorizationstore.Binding, error) {
	if s == nil || s.authority == nil {
		return authorizationstore.Binding{}, errors.New("Channel 授权缺少配置权限校验器")
	}
	if s.humanVerifier == nil {
		return authorizationstore.Binding{}, errors.New("Channel 授权缺少 human principal verifier")
	}
	actor.OwnerUserID = strings.TrimSpace(actor.OwnerUserID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	actor.SessionKey = strings.TrimSpace(actor.SessionKey)
	actor.RoundID = strings.TrimSpace(actor.RoundID)
	actor.LeaseSessionKey = strings.TrimSpace(actor.LeaseSessionKey)
	actor.LeaseRoundID = strings.TrimSpace(actor.LeaseRoundID)
	actor.ContextKind = strings.ToLower(strings.TrimSpace(actor.ContextKind))
	actor.ContextID = strings.TrimSpace(actor.ContextID)
	actor.AuthMethod = strings.TrimSpace(actor.AuthMethod)
	actor.AuthSessionID = strings.TrimSpace(actor.AuthSessionID)
	principal, err := s.humanVerifier.VerifyInteractiveHuman(
		ctx,
		authctx.PrincipalFromContext(ctx),
	)
	if err != nil {
		return authorizationstore.Binding{}, err
	}
	authSessionID := ""
	if principal.SessionID != nil {
		authSessionID = strings.TrimSpace(*principal.SessionID)
	}
	principalUserID := strings.TrimSpace(principal.UserID)
	principalAuthMethod := strings.TrimSpace(principal.AuthMethod)
	if principalUserID != actor.OwnerUserID ||
		principalAuthMethod != actor.AuthMethod ||
		authSessionID != actor.AuthSessionID {
		return authorizationstore.Binding{}, errors.New(
			"Channel 授权当前真人 principal/session 与 runtime 身份不匹配",
		)
	}
	ctx = authctx.WithPrincipal(ctx, principal)
	inspection, err := s.authority.Inspect(
		ctx,
		actor,
		[]string{configurationsvc.DomainChannels},
		false,
	)
	if err != nil {
		return authorizationstore.Binding{}, err
	}
	if inspection == nil ||
		inspection.Authority != configurationsvc.AuthorityOwnerMain ||
		inspection.Context.Kind != configurationsvc.ScopeKindOwner ||
		actor.ContextKind != configurationsvc.ContextKindAgent ||
		actor.ContextID != actor.AgentID {
		return authorizationstore.Binding{}, errors.New(
			"Channel 扫码授权只允许 Nexus 主智能体在自己的 WebSocket 私有 DM 中发起",
		)
	}
	binding := authorizationstore.Binding{
		OwnerUserID:            principalUserID,
		PrincipalUserID:        principalUserID,
		PrincipalRole:          strings.TrimSpace(principal.Role),
		PrincipalAuthMethod:    principalAuthMethod,
		PrincipalAuthSessionID: authSessionID,
		AgentID:                actor.AgentID,
		BusinessSessionKey:     actor.SessionKey,
		RootRoundID:            actor.RoundID,
		RuntimeLeaseSessionKey: actor.LeaseSessionKey,
		RuntimeLeaseRoundID:    actor.LeaseRoundID,
	}
	if actor.LocalSingleUser &&
		principalAuthMethod == authctx.AuthMethodLocal {
		binding.PrincipalRole = "owner"
		binding.PrincipalAuthMethod = "local"
	}
	return binding, nil
}

func requireFlowActorBinding(
	current authorizationstore.Binding,
	stored authorizationstore.Binding,
) error {
	// 原始 root/runtime round 仍保存在 flow、加密人类展示路由和审计中，
	// 验证码提交必须精确匹配它们。模型侧 Status/Cancel/Completion 则允许
	// 同一真人、同一主智能体私聊在后续 active round 恢复 durable flow；
	// 当前 round 的真实性已经由 configuration authority 重新验证。
	if strings.TrimSpace(current.OwnerUserID) != strings.TrimSpace(stored.OwnerUserID) ||
		strings.TrimSpace(current.PrincipalUserID) != strings.TrimSpace(stored.PrincipalUserID) ||
		strings.TrimSpace(current.PrincipalRole) != strings.TrimSpace(stored.PrincipalRole) ||
		strings.TrimSpace(current.PrincipalAuthMethod) != strings.TrimSpace(stored.PrincipalAuthMethod) ||
		strings.TrimSpace(current.PrincipalAuthSessionID) != strings.TrimSpace(stored.PrincipalAuthSessionID) ||
		strings.TrimSpace(current.AgentID) != strings.TrimSpace(stored.AgentID) ||
		strings.TrimSpace(current.BusinessSessionKey) != strings.TrimSpace(stored.BusinessSessionKey) ||
		strings.TrimSpace(current.RuntimeLeaseSessionKey) != strings.TrimSpace(stored.RuntimeLeaseSessionKey) {
		return errors.New("Channel 授权 flow 与当前 principal、主智能体或私有 DM 不匹配")
	}
	return nil
}

func (s *Service) verifyFlowHuman(
	ctx context.Context,
	binding authorizationstore.Binding,
	requireRequestEvidence bool,
) (*authctx.Principal, error) {
	if s == nil || s.humanVerifier == nil {
		return nil, errors.New("Channel 授权缺少 human principal verifier")
	}
	var (
		principal *authctx.Principal
		err       error
	)
	if requireRequestEvidence {
		principal, err = s.humanVerifier.VerifyInteractiveHuman(
			ctx,
			authctx.PrincipalFromContext(ctx),
		)
	} else {
		principal, err = s.humanVerifier.VerifyBoundInteractiveHuman(
			ctx,
			binding.PrincipalUserID,
			binding.PrincipalAuthMethod,
			binding.PrincipalAuthSessionID,
		)
	}
	if err != nil {
		return nil, err
	}
	sessionID := ""
	if principal.SessionID != nil {
		sessionID = strings.TrimSpace(*principal.SessionID)
	}
	if strings.TrimSpace(principal.UserID) != strings.TrimSpace(binding.PrincipalUserID) ||
		strings.TrimSpace(principal.Role) != strings.TrimSpace(binding.PrincipalRole) ||
		strings.TrimSpace(principal.AuthMethod) != strings.TrimSpace(binding.PrincipalAuthMethod) ||
		sessionID != strings.TrimSpace(binding.PrincipalAuthSessionID) {
		return nil, errors.New("Channel 授权原始真人 principal/session 已变化")
	}
	return principal, nil
}
