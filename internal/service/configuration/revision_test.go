// INPUT: 相同配置值的不同 scope/target/version、秘密轮换与历史 revision 格式。
// OUTPUT: 不跨 scope 关联、秘密变更可见、旧 receipt 不伪报为配置变化。
// POS: 稳定配置 revision 的语义边界回归。
package configuration

import (
	"strings"
	"testing"
)

func TestConfigurationRevisionBindsScopeAndSecrets(t *testing.T) {
	service, _, _ := newAuditTestService(t)
	scope := ScopeRef{Kind: ScopeKindOwner, ID: "owner-one"}
	values := map[string]any{"api_key": "secret-one"}
	before, err := service.snapshotRevision(t.Context(), DomainProviders, scope, "provider", 1, values)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"scope", "domain", "target", "version", "secret"} {
		t.Run(scenario, func(t *testing.T) {
			domain, currentScope, target, version := DomainProviders, scope, "provider", int64(1)
			currentValues := map[string]any{"api_key": "secret-one"}
			switch scenario {
			case "scope":
				currentScope.ID = "owner-two"
			case "domain":
				domain = DomainPreferences
			case "target":
				target = "another-provider"
			case "version":
				version++
			case "secret":
				currentValues["api_key"] = "secret-two"
			}
			after, err := service.snapshotRevision(t.Context(), domain, currentScope, target, version, currentValues)
			if err != nil {
				t.Fatal(err)
			}
			if after == before || !strings.HasPrefix(after, durableRevisionPrefix) || strings.Contains(after, "secret") {
				t.Fatal("revision did not bind the changed identity/state without exposing the secret")
			}
		})
	}
}

func TestConfigurationRevisionLegacyReceiptIsIncomparable(t *testing.T) {
	current := DomainSnapshot{Revision: durableRevisionPrefix + strings.Repeat("a", 64)}
	record := AuditRecord{RevisionBefore: "hmac-sha256:legacy-process-key", Status: "reconcile_required"}
	if got := reconciliationEvidence(record, current); got.RevisionRelation != "incomparable" || got.DecisionSource != "human_confirmation_required" {
		t.Fatalf("legacy evidence = %+v", got)
	}
	record.RevisionAfter = current.Revision
	if got := reconciliationEvidence(record, current); got.RevisionRelation != "matches_recorded_after" {
		t.Fatalf("current-format after revision was not usable: %+v", got)
	}
	record.RevisionAfter = durableRevisionPrefix + strings.Repeat("b", 64)
	if got := reconciliationEvidence(record, current); got.RevisionRelation != "different" {
		t.Fatalf("current-format difference was hidden: %+v", got)
	}
}
