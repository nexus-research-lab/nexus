// INPUT: 同一 owner 下的 host/private 审计与 member 角色主智能体调用。
// OUTPUT: 未指定域的历史查询排除 host，显式 host 查询继续 fail-closed。
// POS: 对话配置审计读取的宿主角色边界集成测试。
package configuration_test

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type fixedConfigurationRoleResolver struct {
	role string
}

func (r fixedConfigurationRoleResolver) ResolveActivePrincipalRole(context.Context, string) (string, error) {
	return r.role, nil
}

func TestMemberMainAuditListingExcludesHostConfiguration(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	db, err := storage.OpenDB(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, record := range []struct {
		requestID string
		domain    string
	}{
		{requestID: "audit-host-boundary", domain: configurationsvc.DomainHost},
		{requestID: "audit-private-boundary", domain: configurationsvc.DomainPreferences},
	} {
		if _, err = db.ExecContext(
			t.Context(),
			`INSERT INTO configuration_changes (
				request_id, owner_user_id, actor_agent_id, domain, operation,
				scope_kind, scope_id, authority, status
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.requestID,
			fixture.main.OwnerUserID,
			fixture.main.AgentID,
			record.domain,
			"inspect",
			configurationsvc.ScopeKindOwner,
			fixture.main.OwnerUserID,
			configurationsvc.AuthorityOwnerMain,
			"success",
		); err != nil {
			t.Fatal(err)
		}
	}

	fixture.services.Configuration.SetPrincipalVerifiers(
		configurationTestPrincipalVerifier{},
		fixedConfigurationRoleResolver{role: authctx.RoleMember},
	)
	memberCtx := authctx.WithPrincipal(t.Context(), &authctx.Principal{
		UserID: fixture.main.OwnerUserID, Role: authctx.RoleMember,
		AuthMethod: authctx.AuthMethodPassword,
	})
	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:audit-boundary",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	records, err := fixture.services.Configuration.ListChanges(memberCtx, actor, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Domain == configurationsvc.DomainHost {
			t.Fatalf("member main Agent received host audit record: %+v", record)
		}
	}
	if _, err = fixture.services.Configuration.ListChanges(
		memberCtx,
		actor,
		configurationsvc.DomainHost,
		20,
	); err == nil {
		t.Fatal("member main Agent explicitly read host audit history")
	}
}
