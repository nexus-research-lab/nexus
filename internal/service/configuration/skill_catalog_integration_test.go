// INPUT: human-only 本地导入形成的 owner Skill、主智能体对话计划与并发来源更新。
// OUTPUT: 路径脱敏、catalog_version CAS、过期计划拒绝与删除写后不存在证明。
// POS: nexuscfg Skill 全局目录控制的端到端边界回归测试。
package configuration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
)

func TestSkillCatalogConversationUsesStableVersionAndNeverExposesLocalPath(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	const skillName = "catalog-version-skill"
	localSkillPath := filepath.Join(t.TempDir(), "private-local-skill")
	if err := os.MkdirAll(localSkillPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(localSkillPath, "SKILL.md"),
		[]byte(`---
name: catalog-version-skill
title: Catalog Version Skill
description: catalog CAS test
---

# Catalog Version Skill
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.services.Skills.ImportLocalPath(
		fixture.ownerCtx,
		localSkillPath,
	); err != nil {
		t.Fatal(err)
	}

	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:skill-catalog",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainSkills},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(inspection.Domains[configurationsvc.DomainSkills].Values)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), localSkillPath) ||
		strings.Contains(string(payload), `"source_ref"`) {
		t.Fatalf("Skill inspect leaked local source path/reference: %s", payload)
	}

	plan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "delete", Target: skillName,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	state, err := fixture.services.Skills.GetCatalogSkillState(
		fixture.ownerCtx,
		skillName,
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.StateVersion != state.CatalogVersion || !plan.RequiresConfirmation {
		t.Fatalf("Skill delete plan did not bind catalog version/approval: %+v", plan)
	}

	concurrentSkillPath := filepath.Join(t.TempDir(), "concurrent-local-skill")
	if err = os.MkdirAll(concurrentSkillPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(
		filepath.Join(concurrentSkillPath, "SKILL.md"),
		[]byte(`---
name: concurrent-catalog-skill
title: Concurrent Catalog Skill
description: advances catalog version
---

# Concurrent Catalog Skill
`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Skills.ImportLocalPath(
		fixture.ownerCtx,
		concurrentSkillPath,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			RequestID: "skill-catalog-delete-stale-001",
			Domain:    configurationsvc.DomainSkills, Operation: "delete", Target: skillName,
			ExpectedRevision: plan.CurrentRevision,
			PlanDigest:       plan.PlanDigest,
		},
	); err == nil {
		t.Fatal("stale Skill catalog plan must fail closed")
	}
	if preserved, getErr := fixture.services.Skills.GetCatalogSkillState(
		fixture.ownerCtx,
		skillName,
	); getErr != nil || !preserved.Exists {
		t.Fatalf("stale Skill plan removed target: state=%+v err=%v", preserved, getErr)
	}

	plan, err = fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "delete", Target: skillName,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := configurationsvc.ChangeRequest{
		RequestID: "skill-catalog-delete-001",
		Domain:    configurationsvc.DomainSkills, Operation: "delete", Target: skillName,
		ExpectedRevision: plan.CurrentRevision,
		PlanDigest:       plan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		request,
		plan,
	)
	result, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(result.Checks, "skill_catalog_target_verified") ||
		!hasConfigurationCheck(result.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("Skill delete lacked catalog absence/version proof: %+v", result)
	}
	deleted, err := fixture.services.Skills.GetCatalogSkillState(
		fixture.ownerCtx,
		skillName,
	)
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Exists {
		t.Fatalf("Skill still exists after verified delete: %+v", deleted)
	}
}
