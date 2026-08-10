// INPUT: 配置操作与不同敏感度的计划输入。
// OUTPUT: 高影响、敏感与破坏性操作必须批准，普通资料操作不误拦截。
// POS: configuration 风险分级的单元回归测试。
package configuration

import "testing"

func TestClassifyChangeRiskRequiresHumanApprovalForSensitiveInput(t *testing.T) {
	risk, requires := classifyChangeRisk(OperationDefinition{}, ChangeRequest{
		Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"web_search_api_key":"secret"}`),
	})
	if risk != "sensitive" || !requires {
		t.Fatalf("risk=%q requires=%v, want sensitive/true", risk, requires)
	}
}

func TestClassifyChangeRiskRequiresHumanApprovalForRuntimePreferences(t *testing.T) {
	for _, input := range []string{
		`{"agent_runtime_kind":"nxs"}`,
		`{"runtime_settings":{"nxs":{"setting_sources":["user"]}}}`,
		`{"web_search":{"enabled":true}}`,
		`{"default_agent_options":{"model":"expensive"}}`,
		`{"default_image_model_selection":{"provider":"custom","model":"image"}}`,
	} {
		risk, requires := classifyChangeRisk(OperationDefinition{}, ChangeRequest{
			Domain: DomainPreferences, Operation: "update", Input: []byte(input),
		})
		if risk != "high_risk" || !requires {
			t.Fatalf("input=%s risk=%q requires=%v, want high_risk/true", input, risk, requires)
		}
	}
}

func TestClassifyChangeRiskLeavesBenignPreferencesUnblocked(t *testing.T) {
	risk, requires := classifyChangeRisk(OperationDefinition{}, ChangeRequest{
		Domain: DomainPreferences, Operation: "update",
		Input: []byte(`{"agent_sdk_diagnostics_enabled":true}`),
	})
	if risk != "normal" || requires {
		t.Fatalf("risk=%q requires=%v, want normal/false", risk, requires)
	}
}

func TestClassifyChangeRiskDistinguishesHighRiskFromDestructive(t *testing.T) {
	risk, requires := classifyChangeRisk(OperationDefinition{
		RequiresConfirmation: true,
	}, ChangeRequest{Domain: DomainChannels, Operation: "upsert"})
	if risk != "high_risk" || !requires {
		t.Fatalf("upsert risk=%q requires=%v, want high_risk/true", risk, requires)
	}

	risk, requires = classifyChangeRisk(OperationDefinition{
		RequiresConfirmation: true,
	}, ChangeRequest{Domain: DomainChannels, Operation: "delete_config"})
	if risk != "destructive" || !requires {
		t.Fatalf("delete risk=%q requires=%v, want destructive/true", risk, requires)
	}
}

func TestCatalogRequiresHumanApprovalForExternalAndExecutableChanges(t *testing.T) {
	for _, reference := range []struct {
		domain    string
		operation string
	}{
		{DomainProviders, "create"},
		{DomainAgents, "create"},
		{DomainChannels, "upsert"},
		{DomainConnectors, "connect"},
		{DomainSkills, "import_git"},
		{DomainSkills, "import_url"},
		{DomainSkills, "import_skills_sh"},
		{DomainSkills, "update_source"},
		{DomainSkills, "install"},
		{DomainSkills, "update_single"},
		{DomainAgents, "update_self_runtime"},
		{DomainRooms, "update_profile"},
	} {
		definition, err := definitionFor(reference.domain)
		if err != nil {
			t.Fatal(err)
		}
		operation, err := operationFor(definition, reference.operation)
		if err != nil {
			t.Fatal(err)
		}
		if !operation.RequiresConfirmation {
			t.Fatalf("%s.%s must require human approval", reference.domain, reference.operation)
		}
	}
}
