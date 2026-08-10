package configuration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

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
