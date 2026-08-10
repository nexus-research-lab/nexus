package configuration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

func TestConsumeHumanApprovalIsBoundExpiringAndOneTime(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	service := &Service{
		humanApprovals: make(map[string]humanApprovalRecord),
		approvalNow:    func() time.Time { return now },
	}
	actor := &resolvedActor{
		Actor: Actor{
			OwnerUserID: "owner-a", AgentID: "agent-a",
			SessionKey: "session-a", RoundID: "round-a",
			LeaseSessionKey: "runtime-session-a", LeaseRoundID: "runtime-round-a",
			ContextKind: ContextKindAgent, ContextID: "agent-a",
			PrincipalRole: "owner", AuthMethod: "password",
		},
		Authority: AuthorityAgentSelf,
		Context:   ScopeRef{Kind: ScopeKindAgent, ID: "agent-a"},
	}
	request := ChangeRequest{
		RequestID: "approval-once-01",
		Domain:    DomainAgents, Operation: "update_self_profile",
		PlanDigest: "hmac:plan", ExpectedRevision: "sha256:before",
	}
	plan := ChangePlan{
		Domain: DomainAgents, Operation: "update_self_profile",
		Scope:      ScopeRef{Kind: ScopeKindAgent, ID: "agent-a"},
		PlanDigest: "hmac:plan", CurrentRevision: "sha256:before",
	}
	key := humanApprovalKey(actor.OwnerUserID, request.RequestID)
	record := humanApprovalRecord{
		PermissionRequestID: "perm-approval-once-01",
		OwnerUserID:         actor.OwnerUserID, PrincipalRole: actor.PrincipalRole,
		PrincipalAuthMethod: actor.AuthMethod,
		AgentID:             actor.AgentID, SessionKey: actor.SessionKey, RoundID: actor.RoundID,
		LeaseSessionKey: actor.LeaseSessionKey, LeaseRoundID: actor.LeaseRoundID,
		ContextKind: actor.ContextKind, ContextID: actor.ContextID,
		Scope: plan.Scope, RequestID: request.RequestID,
		PlanDigest: plan.PlanDigest, ExpectedRevision: plan.CurrentRevision,
		Domain: plan.Domain, Operation: plan.Operation, Target: plan.Target,
		ApprovedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Minute),
	}

	expired := record
	expired.ExpiresAt = now
	service.humanApprovals[key] = expired
	if _, err := service.consumeHumanApproval(actor, request, plan); err == nil ||
		!strings.Contains(err.Error(), "尚未获得") {
		t.Fatalf("expired approval error = %v", err)
	}
	if _, exists := service.humanApprovals[key]; exists {
		t.Fatal("expired approval was not purged")
	}

	wrongSession := record
	wrongSession.SessionKey = "session-b"
	service.humanApprovals[key] = wrongSession
	if _, err := service.consumeHumanApproval(actor, request, plan); err == nil ||
		!strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("cross-session approval error = %v", err)
	}
	if _, exists := service.humanApprovals[key]; !exists {
		t.Fatal("mismatched caller consumed another session's approval")
	}

	wrongLease := record
	wrongLease.LeaseRoundID = "runtime-round-b"
	service.humanApprovals[key] = wrongLease
	if _, err := service.consumeHumanApproval(actor, request, plan); err == nil ||
		!strings.Contains(err.Error(), "不匹配") {
		t.Fatalf("cross-lease approval error = %v", err)
	}
	if _, exists := service.humanApprovals[key]; !exists {
		t.Fatal("mismatched caller consumed another runtime lease's approval")
	}

	service.humanApprovals[key] = record
	consumed, err := service.consumeHumanApproval(actor, request, plan)
	if err != nil {
		t.Fatalf("consume valid approval: %v", err)
	}
	if consumed == nil || consumed.PermissionRequestID != record.PermissionRequestID {
		t.Fatalf("consumed approval = %+v", consumed)
	}
	if _, exists := service.humanApprovals[key]; exists {
		t.Fatal("valid approval was not consumed")
	}
	if _, err = service.consumeHumanApproval(actor, request, plan); err == nil {
		t.Fatal("one-time approval was replayed")
	}
}

func TestConfigurationApplyApprovalToolNameIsExactLeaf(t *testing.T) {
	for _, allowed := range []string{
		"apply_nexus_configuration_change",
		"mcp__nexus_config__apply_nexus_configuration_change",
		"nexus_config.apply_nexus_configuration_change",
		"nexus_config/apply_nexus_configuration_change",
	} {
		if !isConfigurationApplyApprovalTool(allowed) {
			t.Fatalf("expected configuration apply tool %q to be accepted", allowed)
		}
	}
	for _, rejected := range []string{
		"",
		"apply_nexus_configuration_change_extra",
		"forgedapply_nexus_configuration_change",
		"mcp__nexus_config__plan_nexus_configuration_change",
	} {
		if isConfigurationApplyApprovalTool(rejected) {
			t.Fatalf("forged configuration approval tool %q was accepted", rejected)
		}
	}
}

func TestActorFromHumanApprovalKeepsRoomBusinessAndLeaseIdentitySeparate(t *testing.T) {
	service := &Service{}
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner-a", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})

	dmActor, err := service.actorFromHumanApproval(ctx, permissionctx.HumanToolApproval{
		RuntimeSessionKey:  "agent:agent-a:dm:main",
		DispatchSessionKey: "agent:agent-a:dm:main",
		Route: permissionctx.RouteContext{
			AgentID: "agent-a", RoundID: "dm-round-a",
		},
	})
	if err != nil {
		t.Fatalf("DM approval actor: %v", err)
	}
	if dmActor.SessionKey != dmActor.LeaseSessionKey ||
		dmActor.RoundID != dmActor.LeaseRoundID {
		t.Fatalf("DM business and lease identity must match: %+v", dmActor)
	}

	roomActor, err := service.actorFromHumanApproval(ctx, permissionctx.HumanToolApproval{
		RuntimeSessionKey:  "room:room-a:conversation-a:agent-a",
		DispatchSessionKey: "room:room-a:conversation-a",
		Route: permissionctx.RouteContext{
			AgentID: "agent-a", RoundID: "root-round-a", AgentRoundID: "agent-round-a",
			RoomID: "room-a", ConversationID: "conversation-a",
		},
	})
	if err != nil {
		t.Fatalf("Room approval actor: %v", err)
	}
	if roomActor.SessionKey != "room:room-a:conversation-a" ||
		roomActor.RoundID != "root-round-a" ||
		roomActor.LeaseSessionKey != "room:room-a:conversation-a:agent-a" ||
		roomActor.LeaseRoundID != "agent-round-a" {
		t.Fatalf("Room business and lease identity were conflated: %+v", roomActor)
	}

	_, err = service.actorFromHumanApproval(ctx, permissionctx.HumanToolApproval{
		RuntimeSessionKey:  "room:room-a:conversation-a:agent-a",
		DispatchSessionKey: "room:room-a:conversation-a",
		Route: permissionctx.RouteContext{
			AgentID: "agent-a", RoundID: "root-round-a", RoomID: "room-a",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "lease") {
		t.Fatalf("Room approval without Agent slot round must fail closed: %v", err)
	}
}
