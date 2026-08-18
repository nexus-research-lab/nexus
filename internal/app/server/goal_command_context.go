// INPUT: runtime Agent/session/source 与 runtime-owned Goal authority。
// OUTPUT: 持久负责人 exact Goal-only snapshot、窄 user-retarget 来源及 canonical Goal session identity。
// POS: round-scoped Goal command Actor 装配边界；私有 Goal authority 不泄漏为 Execution authority。
package server

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalcontract "github.com/nexus-research-lab/nexus/internal/runtimecommand/goal/contract"
)

type goalCommandMutationAuthorityResolver interface {
	CurrentModelMutationAuthority(context.Context, string, string, string) (*protocol.Goal, error)
}

func resolveGoalCommandMutationAuthority(
	ctx context.Context,
	svc goalcontract.Service,
	sessionKey string,
	sourceContextType string,
	agentValue *protocol.Agent,
	roundAuthority *runtimectx.GoalAuthorityState,
) *runtimectx.GoalAuthorityState {
	if roundAuthority == nil {
		roundAuthority = runtimectx.NewGoalAuthorityState("", 0, "")
	}
	if _, ok := roundAuthority.Load(); ok || agentValue == nil ||
		!allowsDurableGoalOwnerAuthority(sessionKey, sourceContextType) {
		return roundAuthority
	}
	resolver, ok := svc.(goalCommandMutationAuthorityResolver)
	if !ok || resolver == nil {
		return roundAuthority
	}
	item, err := resolver.CurrentModelMutationAuthority(
		ctx,
		sessionKey,
		strings.TrimSpace(agentValue.OwnerUserID),
		strings.TrimSpace(agentValue.AgentID),
	)
	if err != nil || item == nil || strings.TrimSpace(item.ID) == "" || item.ObjectiveRevision() <= 0 {
		return roundAuthority
	}
	return runtimectx.NewGoalAuthorityState(item.ID, item.ObjectiveRevision(), "")
}

func allowsDurableGoalOwnerAuthority(sessionKey string, sourceContextType string) bool {
	if protocol.IsRoomSharedSessionKey(sessionKey) {
		switch strings.TrimSpace(sourceContextType) {
		case "room", "room_handoff":
			return true
		default:
			return false
		}
	}
	return strings.TrimSpace(sourceContextType) == "agent"
}

func allowsTrustedUserGoalRetarget(sourceContextType string) bool {
	switch strings.TrimSpace(sourceContextType) {
	case "agent", "room":
		return true
	default:
		return false
	}
}

func resolveGoalCommandSessionKey(sessionKey string, sourceContextType string) string {
	normalized := strings.TrimSpace(sessionKey)
	if normalized == "" || strings.TrimSpace(sourceContextType) != "room" {
		return normalized
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return normalized
	}
	if parsed.Kind == protocol.SessionKeyKindAgent && parsed.ChatType == "group" && strings.TrimSpace(parsed.Ref) != "" {
		return protocol.BuildRoomSharedSessionKey(parsed.Ref)
	}
	return normalized
}
