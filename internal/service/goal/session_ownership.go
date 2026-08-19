// INPUT: authenticated owner、canonical Goal session key 与仅由可信 runtime 注入的可选 Agent identity。
// OUTPUT: owner-scoped session 存在性证明与经服务端校验的 Room creator/lead Agent identity。
// POS: Goal create 与 app-server no-current set 共用的跨域 session ownership 边界。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// GoalSessionOwnershipRequest 是 Goal 服务向应用装配层请求的 owner-scoped
// session 证明。TrustedAgentID 只能来自 runtime/app context，不能来自 JSON。
type GoalSessionOwnershipRequest struct {
	OwnerUserID    string
	SessionKey     string
	TrustedAgentID string
}

// GoalSessionOwnershipProof 返回已经由 Agent/Room 服务验证的可信身份。
type GoalSessionOwnershipProof struct {
	TrustedAgentID   string
	TrustedAgentName string
}

// GoalSessionOwnershipVerifier 由应用层用 owner-scoped Agent/Room 服务实现。
type GoalSessionOwnershipVerifier interface {
	VerifyGoalSessionOwnership(
		context.Context,
		GoalSessionOwnershipRequest,
	) (GoalSessionOwnershipProof, error)
}

// SetSessionOwnershipVerifier 注入 Goal create 的持久 session ownership 证明器。
func (s *Service) SetSessionOwnershipVerifier(verifier GoalSessionOwnershipVerifier) {
	s.sessionOwnership = verifier
}

func (s *Service) verifyGoalSessionOwnership(
	ctx context.Context,
	sessionKey string,
	requestedOwnerUserID string,
	trustedAgentID string,
) (string, string, string, error) {
	ownerUserID := strings.TrimSpace(requestedOwnerUserID)
	if ownerUserID == "" {
		ownerUserID = authctx.OwnerUserID(ctx)
	}
	trustedAgentID = strings.TrimSpace(trustedAgentID)
	if s.sessionOwnership == nil {
		// Focused domain tests construct the Goal service without the app graph.
		// The production server always installs the owner-scoped verifier.
		return ownerUserID, trustedAgentID, "", nil
	}
	if authenticatedOwner, ok := authctx.CurrentUserID(ctx); ok &&
		strings.TrimSpace(authenticatedOwner) != ownerUserID {
		return "", "", "", fmt.Errorf(
			"%w: Goal owner does not match the authenticated owner",
			ErrGoalForbidden,
		)
	}
	proof, err := s.sessionOwnership.VerifyGoalSessionOwnership(ctx, GoalSessionOwnershipRequest{
		OwnerUserID:    ownerUserID,
		SessionKey:     strings.TrimSpace(sessionKey),
		TrustedAgentID: trustedAgentID,
	})
	if err != nil {
		return "", "", "", fmt.Errorf(
			"%w: target Goal session is not owned by the authenticated owner",
			ErrGoalForbidden,
		)
	}
	proofAgentID := strings.TrimSpace(proof.TrustedAgentID)
	if proofAgentID != trustedAgentID {
		return "", "", "", fmt.Errorf(
			"%w: Goal session proof returned a different runtime Agent identity",
			ErrGoalForbidden,
		)
	}
	return ownerUserID,
		proofAgentID,
		strings.TrimSpace(proof.TrustedAgentName),
		nil
}
