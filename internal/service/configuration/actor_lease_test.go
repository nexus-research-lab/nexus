// INPUT: 配置调用的角色、业务身份与 runtime lease。
// OUTPUT: 管理员权限隔离、会话不可转移和活跃 round capability 验证。
// POS: configuration 身份与权限边界单元测试。
package configuration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestRequireActiveRoundUsesDMAndRoomRuntimeLease(t *testing.T) {
	tests := []struct {
		name            string
		sessionKey      string
		roundID         string
		leaseSessionKey string
		leaseRoundID    string
	}{
		{
			name: "dm",
			sessionKey: protocol.BuildAgentSessionKey(
				"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
			),
			roundID: "dm-round-a",
			leaseSessionKey: protocol.BuildAgentSessionKey(
				"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
			),
			leaseRoundID: "dm-round-a",
		},
		{
			name:       "room agent slot",
			sessionKey: protocol.BuildRoomSharedSessionKey("conversation-a"),
			roundID:    "root-round-a",
			leaseSessionKey: protocol.BuildRoomAgentSessionKey(
				"conversation-a", "agent-a", protocol.RoomTypeGroup,
			),
			leaseRoundID: "agent-round-a",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := runtimectx.NewManager()
			if err := manager.StartRound(t.Context(), test.leaseSessionKey, test.leaseRoundID, nil); err != nil {
				t.Fatalf("start runtime lease: %v", err)
			}
			service := &Service{runtime: manager}
			actor := Actor{
				SessionKey: test.sessionKey, RoundID: test.roundID,
				LeaseSessionKey: test.leaseSessionKey, LeaseRoundID: test.leaseRoundID,
				RoundLeaseRequired: true,
			}
			if err := service.requireActiveRound(actor); err != nil {
				t.Fatalf("active lease rejected: %v", err)
			}

			stale := actor
			stale.LeaseRoundID += "-stale"
			if err := service.requireActiveRound(stale); err == nil ||
				!strings.Contains(err.Error(), "已结束") {
				t.Fatalf("stale lease error = %v", err)
			}

			manager.MarkRoundFinished(test.leaseSessionKey, test.leaseRoundID)
			if err := service.requireActiveRound(actor); err == nil ||
				!strings.Contains(err.Error(), "已结束") {
				t.Fatalf("finished lease error = %v", err)
			}
		})
	}
}

func TestValidateAgentRuntimeIdentityRejectsTransferAcrossDMs(t *testing.T) {
	dmSession := protocol.BuildAgentSessionKey(
		"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "dm-a", "",
	)
	actor := Actor{
		AgentID: "agent-a", SessionKey: dmSession, RoundID: "round-a",
		LeaseSessionKey: dmSession, LeaseRoundID: "round-a",
		RoundLeaseRequired: true,
	}
	if err := validateAgentRuntimeIdentity(actor); err != nil {
		t.Fatalf("valid DM identity rejected: %v", err)
	}

	swappedSession := actor
	swappedSession.LeaseSessionKey = protocol.BuildAgentSessionKey(
		"agent-a", protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "dm-b", "",
	)
	if err := validateAgentRuntimeIdentity(swappedSession); err == nil ||
		!strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("swapped DM lease error = %v", err)
	}

	external := actor
	external.SessionKey = protocol.BuildAgentSessionKey(
		"agent-a", "telegram", protocol.RoomTypeDM, "chat-a", "",
	)
	external.LeaseSessionKey = external.SessionKey
	if err := validateAgentRuntimeIdentity(external); err == nil ||
		!strings.Contains(err.Error(), "WebSocket") {
		t.Fatalf("external channel lease error = %v", err)
	}
}

func TestValidateRoomRuntimeIdentityRejectsTransferAcrossAgentSlots(t *testing.T) {
	contextValue := &protocol.ConversationContextAggregate{
		Room: protocol.RoomRecord{ID: "room-a", RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{
			ID: "conversation-a", RoomID: "room-a",
		},
	}
	actor := Actor{
		AgentID: "agent-a", ConversationID: "conversation-a",
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-a"),
		LeaseSessionKey: protocol.BuildRoomAgentSessionKey(
			"conversation-a", "agent-a", protocol.RoomTypeGroup,
		),
		RoundLeaseRequired: true,
	}
	if err := validateRoomRuntimeIdentityValue(actor, "room-a", contextValue); err != nil {
		t.Fatalf("valid Room identity rejected: %v", err)
	}

	swappedAgent := actor
	swappedAgent.LeaseSessionKey = protocol.BuildRoomAgentSessionKey(
		"conversation-a", "agent-b", protocol.RoomTypeGroup,
	)
	if err := validateRoomRuntimeIdentityValue(swappedAgent, "room-a", contextValue); err == nil ||
		!strings.Contains(err.Error(), "Agent slot") {
		t.Fatalf("swapped Room Agent lease error = %v", err)
	}

	swappedConversation := actor
	swappedConversation.LeaseSessionKey = protocol.BuildRoomAgentSessionKey(
		"conversation-b", "agent-a", protocol.RoomTypeGroup,
	)
	if err := validateRoomRuntimeIdentityValue(swappedConversation, "room-a", contextValue); err == nil ||
		!strings.Contains(err.Error(), "Agent slot") {
		t.Fatalf("swapped Room conversation lease error = %v", err)
	}
}

func TestOwnerMainKeepsHumanAdminBoundary(t *testing.T) {
	memberMain := &resolvedActor{
		Actor: Actor{
			OwnerUserID:   "member-user",
			AgentID:       "member-main",
			PrincipalRole: authctx.RoleMember,
			AuthMethod:    authctx.AuthMethodPassword,
		},
		Authority: AuthorityOwnerMain,
		Context:   ScopeRef{Kind: ScopeKindOwner, ID: "member-user"},
	}
	hostDefinition, err := definitionFor(DomainHost)
	if err != nil {
		t.Fatal(err)
	}
	if access := accessFor(memberMain, hostDefinition); access.CanRead {
		t.Fatalf("member main Agent unexpectedly received host access: %+v", access)
	}
	providerDefinition, err := definitionFor(DomainProviders)
	if err != nil {
		t.Fatal(err)
	}
	if access := accessFor(memberMain, providerDefinition); !access.CanRead {
		t.Fatalf("member main Agent should still manage its private Provider scope: %+v", access)
	}
	principal := authctx.PrincipalFromContext(scopedContext(context.Background(), memberMain.Actor))
	if principal == nil || principal.Role != authctx.RoleMember {
		t.Fatalf("configuration context elevated member role: %+v", principal)
	}

	adminMain := *memberMain
	adminMain.PrincipalRole = authctx.RoleAdmin
	if access := accessFor(&adminMain, hostDefinition); !access.CanRead {
		t.Fatalf("admin main Agent should receive host access: %+v", access)
	}
	localMain := *memberMain
	localMain.OwnerUserID = authctx.SystemUserID
	localMain.Context.ID = authctx.SystemUserID
	localMain.PrincipalRole = authctx.RoleOwner
	localMain.AuthMethod = authctx.AuthMethodLocal
	localMain.LocalSingleUser = true
	if access := accessFor(&localMain, hostDefinition); !access.CanRead {
		t.Fatalf("local single-user main Agent should receive host access: %+v", access)
	}
}

func TestRuntimeCapabilityBindsStableSessionToActiveRound(t *testing.T) {
	manager := runtimectx.NewManager()
	service := &Service{
		runtime:                    manager,
		runtimeCapabilities:        make(map[string]*runtimeCapabilityRecord),
		runtimeCapabilityBySession: make(map[string]string),
		runtimeCapabilityNow:       func() time.Time { return time.Now().UTC() },
	}
	actor := Actor{
		OwnerUserID: "owner-a", AgentID: "agent-a",
		LeaseSessionKey: "session-a", LeaseRoundID: "round-a",
		RoundLeaseRequired: true,
	}
	token, err := service.IssueRuntimeCapability(actor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveRuntimeCapability(token); err == nil ||
		!strings.Contains(err.Error(), "尚未开始") {
		t.Fatalf("inactive capability error = %v", err)
	}
	if err = manager.StartRound(context.Background(), actor.LeaseSessionKey, actor.LeaseRoundID, nil); err != nil {
		t.Fatal(err)
	}
	resolved, err := service.ResolveRuntimeCapability(token)
	if err != nil || resolved.AgentID != actor.AgentID {
		t.Fatalf("resolved actor = %+v, err=%v", resolved, err)
	}

	next := actor
	next.LeaseRoundID = "round-b"
	nextToken, err := service.IssueRuntimeCapability(next)
	if err != nil {
		t.Fatal(err)
	}
	if nextToken != token {
		t.Fatalf("same runtime session rotated token: %q != %q", nextToken, token)
	}
	if err = manager.StartRound(context.Background(), next.LeaseSessionKey, next.LeaseRoundID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ResolveRuntimeCapability(token); err == nil ||
		!strings.Contains(err.Error(), "并发 round") {
		t.Fatalf("concurrent rounds must fail closed, err=%v", err)
	}
	manager.MarkRoundFinished(actor.LeaseSessionKey, actor.LeaseRoundID)
	resolved, err = service.ResolveRuntimeCapability(token)
	if err != nil || resolved.LeaseRoundID != next.LeaseRoundID {
		t.Fatalf("successor actor = %+v, err=%v", resolved, err)
	}
}
