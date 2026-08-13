// INPUT: 当前 runtime Agent/owner/session/round、可信用户来源、runtime-owned Goal authority 与 Goal 服务。
// OUTPUT: 绑定当前 owner、持久负责人 exact Goal-only 快照、窄 retarget 来源及可选 Execution capability 的 MCP server。
// POS: nexus_goal MCP 的应用装配入口。
package server

import (
	"context"
	"strings"
	"sync/atomic"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/config"
	goalmcp "github.com/nexus-research-lab/nexus/internal/mcp/goal"
	goalmcpcontract "github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

type goalMCPMutationAuthorityResolver interface {
	CurrentModelMutationAuthority(
		context.Context,
		string,
		string,
		string,
	) (*protocol.Goal, error)
}

func newGoalMCPBuilder(
	cfg config.Config,
	svc goalmcpcontract.Service,
) func(context.Context, *protocol.Agent, string, string, string, string, string, *atomic.Int64, sdkpermission.Mode) map[string]sdkmcp.ServerConfig {
	return func(
		ctx context.Context,
		agentValue *protocol.Agent,
		sessionKey string,
		roundID string,
		sourceContextType string,
		sourceContextID string,
		sourceContextLabel string,
		_ *atomic.Int64,
		permissionMode sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		goalSessionKey := resolveGoalMCPSessionKey(sessionKey, sourceContextType)
		if !cfg.GoalEnabled || svc == nil || goalSessionKey == "" {
			return nil
		}
		roundAuthority := runtimectx.GoalAuthorityStateFromContext(ctx)
		if roundAuthority == nil {
			roundAuthority = runtimectx.NewGoalAuthorityState("", 0, "")
		}
		authority := resolveGoalMCPMutationAuthority(
			ctx,
			svc,
			goalSessionKey,
			sourceContextType,
			agentValue,
			roundAuthority,
		)
		sctx := goalmcpcontract.ServerContext{
			CurrentSessionKey: goalSessionKey,
			CurrentRoundID:    strings.TrimSpace(roundID),
			GoalAuthority:     authority,
			AllowUserRetarget: allowsTrustedUserGoalRetarget(sourceContextType),
			PlanMode: runtimepermission.NormalizeMode(permissionMode) ==
				sdkpermission.ModePlan,
		}
		if agentValue != nil {
			sctx.CurrentAgentID = strings.TrimSpace(agentValue.AgentID)
			sctx.OwnerUserID = strings.TrimSpace(agentValue.OwnerUserID)
		}
		return map[string]sdkmcp.ServerConfig{
			goalmcpcontract.ServerName: sdkmcp.SDKServerConfig{
				Name:     goalmcpcontract.ServerName,
				Instance: goalmcp.NewServer(svc, sctx),
			},
		}
	}
}

// resolveGoalMCPMutationAuthority preserves an existing host-minted round
// capability. Otherwise it gives the durable Goal owner one private, exact
// revision snapshot for nexus_goal only. The private state is deliberately not
// written back to the runtime context, so nexus_execution cannot interpret
// ownership as ambient WorkGraph mutation authority.
func resolveGoalMCPMutationAuthority(
	ctx context.Context,
	svc goalmcpcontract.Service,
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
	resolver, ok := svc.(goalMCPMutationAuthorityResolver)
	if !ok || resolver == nil {
		return roundAuthority
	}
	item, err := resolver.CurrentModelMutationAuthority(
		ctx,
		sessionKey,
		strings.TrimSpace(agentValue.OwnerUserID),
		strings.TrimSpace(agentValue.AgentID),
	)
	if err != nil || item == nil || strings.TrimSpace(item.ID) == "" ||
		item.ObjectiveRevision() <= 0 {
		return roundAuthority
	}
	return runtimectx.NewGoalAuthorityState(
		item.ID,
		item.ObjectiveRevision(),
		"",
	)
}

func allowsDurableGoalOwnerAuthority(
	sessionKey string,
	sourceContextType string,
) bool {
	sourceContextType = strings.TrimSpace(sourceContextType)
	if protocol.IsRoomSharedSessionKey(sessionKey) {
		switch sourceContextType {
		case "room", "room_handoff":
			return true
		default:
			return false
		}
	}
	return sourceContextType == "agent"
}

func allowsTrustedUserGoalRetarget(sourceContextType string) bool {
	switch strings.TrimSpace(sourceContextType) {
	case "agent", "room":
		return true
	default:
		return false
	}
}

func resolveGoalMCPSessionKey(sessionKey string, sourceContextType string) string {
	normalized := strings.TrimSpace(sessionKey)
	if normalized == "" || strings.TrimSpace(sourceContextType) != "room" {
		return normalized
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return normalized
	}
	if parsed.Kind == protocol.SessionKeyKindAgent &&
		parsed.ChatType == "group" &&
		strings.TrimSpace(parsed.Ref) != "" {
		return protocol.BuildRoomSharedSessionKey(parsed.Ref)
	}
	return normalized
}
