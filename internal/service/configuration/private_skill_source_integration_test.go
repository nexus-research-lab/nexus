// INPUT: owner-main 对话、原生 secret 批准、私有 registry 与并发 catalog 写。
// OUTPUT: 私有来源创建/轮换/导入/删除的 CAS、写后核验、角色隔离与审计脱敏证明。
// POS: 私有 Skill 来源进入 conversational configuration 闭环的端到端回归测试。
package configuration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	skillsvc "github.com/nexus-research-lab/nexus/internal/service/skills"
)

func TestPrivateSkillSourceConversationUsesSecretsCASAndWriteAfterRead(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	const (
		initialToken = "conversation-private-token-v1"
		rotatedToken = "conversation-private-token-v2"
		skillID      = "conversation-private-skill"
	)
	archive := privateSkillArchive(t, skillID)
	archiveSum := sha256.Sum256(archive)
	var tokenState struct {
		sync.RWMutex
		value string
	}
	tokenState.value = initialToken

	mux := http.NewServeMux()
	var registry *httptest.Server
	mux.HandleFunc("/registry/api/skills", func(writer http.ResponseWriter, request *http.Request) {
		tokenState.RLock()
		expectedToken := tokenState.value
		tokenState.RUnlock()
		if request.Header.Get("Authorization") != "Bearer "+expectedToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		response := map[string]any{
			"skills": []map[string]any{{
				"id": skillID, "name": skillID, "title": "Conversation Private Skill",
				"description": "private source conversation test", "version": "1.0.0",
				"tags":         []string{"private"},
				"download_url": registry.URL + "/registry/download/skill.zip",
				"sha256":       hex.EncodeToString(archiveSum[:]), "size": len(archive),
				"readme_markdown": "# Conversation Private Skill",
			}},
			"total": 1,
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(response); err != nil {
			t.Errorf("encode private registry response: %v", err)
		}
	})
	mux.HandleFunc("/registry/download/skill.zip", func(writer http.ResponseWriter, request *http.Request) {
		tokenState.RLock()
		expectedToken := tokenState.value
		tokenState.RUnlock()
		if request.Header.Get("Authorization") != "Bearer "+expectedToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(archive)
	})
	registry = httptest.NewServer(mux)
	t.Cleanup(registry.Close)

	actor := configurationsvc.Actor{
		OwnerUserID: fixture.main.OwnerUserID,
		AgentID:     fixture.main.AgentID,
		SessionKey:  "agent:" + fixture.main.AgentID + ":ws:dm:private-skill-source",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   fixture.main.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)

	createInput := json.RawMessage(`{
		"name":"Conversation Registry",
		"url":"` + registry.URL + `/registry",
		"auth_type":"bearer",
		"token":{"$secret":"private-source-create-token"}
	}`)
	createPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "create_private_source", Input: createInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if createPlan.StateVersion <= 0 || !createPlan.RequiresConfirmation ||
		len(createPlan.SecretSlots) != 1 {
		t.Fatalf("private source create plan did not bind catalog/approval/secret: %+v", createPlan)
	}
	createRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-create-001",
		Domain:    configurationsvc.DomainSkills, Operation: "create_private_source", Input: createInput,
		ExpectedRevision: createPlan.CurrentRevision,
		PlanDigest:       createPlan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		createRequest,
		createPlan,
		map[string]string{"private-source-create-token": initialToken},
	)
	created, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		createRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(created.Checks, "skill_private_source_creation_verified") ||
		!hasConfigurationCheck(created.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private source create lacked write-after-read/version proof: %+v", created)
	}
	sourceID := privateSourceID(t, fixture.services.Skills, fixture.ownerCtx)
	state, err := fixture.services.Skills.GetCatalogSourceState(fixture.ownerCtx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Exists || !state.Deletable || state.AuthType != "bearer" ||
		!state.CredentialConfigured || state.CatalogVersion != createPlan.StateVersion+1 {
		t.Fatalf("private source state after create = %+v", state)
	}

	inspection, err := fixture.services.Configuration.Inspect(
		fixture.ownerCtx,
		actor,
		[]string{configurationsvc.DomainSkills},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	inspectionJSON, err := json.Marshal(inspection)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(inspectionJSON, []byte(initialToken)) ||
		!bytes.Contains(inspectionJSON, []byte(`"credential_configured":true`)) {
		t.Fatalf("private source inspect leaked or omitted safe credential state: %s", inspectionJSON)
	}

	tokenState.Lock()
	tokenState.value = rotatedToken
	tokenState.Unlock()
	updateInput := json.RawMessage(`{
		"name":"Conversation Registry Rotated",
		"token":{"$secret":"private-source-rotate-token"}
	}`)
	updatePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: updateInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	updateRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-update-001",
		Domain:    configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: updateInput,
		ExpectedRevision: updatePlan.CurrentRevision,
		PlanDigest:       updatePlan.PlanDigest,
	}
	approveConfigurationTestChangeWithSecrets(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		updateRequest,
		updatePlan,
		map[string]string{"private-source-rotate-token": rotatedToken},
	)
	updated, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		updateRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(updated.Checks, "skill_source_configuration_verified") ||
		!hasConfigurationCheck(updated.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private source update lacked write-after-read/version proof: %+v", updated)
	}

	staleInput := json.RawMessage(`{"enabled":false}`)
	stalePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: staleInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	staleRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-stale-001",
		Domain:    configurationsvc.DomainSkills, Operation: "update_source", Target: sourceID, Input: staleInput,
		ExpectedRevision: stalePlan.CurrentRevision,
		PlanDigest:       stalePlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		staleRequest,
		stalePlan,
	)
	concurrentName := "Changed through HTTP service path"
	if _, err = fixture.services.Skills.UpdateExternalSkillSource(
		fixture.ownerCtx,
		sourceID,
		skillsvc.ExternalSkillSourceRequest{Name: &concurrentName},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		staleRequest,
	); err == nil {
		t.Fatal("private source plan must be invalidated by a non-conversation catalog write")
	}

	importInput := json.RawMessage(`{"skill_id":"` + skillID + `"}`)
	importPlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "import_private", Target: sourceID, Input: importInput,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	importRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-import-001",
		Domain:    configurationsvc.DomainSkills, Operation: "import_private", Target: sourceID, Input: importInput,
		ExpectedRevision: importPlan.CurrentRevision,
		PlanDigest:       importPlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		importRequest,
		importPlan,
	)
	imported, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		importRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(imported.Checks, "skill_catalog_publication_verified") ||
		!hasConfigurationCheck(imported.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private Skill import lacked publication/version proof: %+v", imported)
	}

	deletePlan, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain: configurationsvc.DomainSkills, Operation: "delete_private_source", Target: sourceID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	deleteRequest := configurationsvc.ChangeRequest{
		RequestID: "private-source-delete-001",
		Domain:    configurationsvc.DomainSkills, Operation: "delete_private_source", Target: sourceID,
		ExpectedRevision: deletePlan.CurrentRevision,
		PlanDigest:       deletePlan.PlanDigest,
	}
	approveConfigurationTestChange(
		t,
		fixture.services,
		fixture.ownerCtx,
		actor,
		deleteRequest,
		deletePlan,
	)
	deleted, err := fixture.services.Configuration.ApplyChange(
		fixture.ownerCtx,
		actor,
		deleteRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConfigurationCheck(deleted.Checks, "skill_private_source_deletion_verified") ||
		!hasConfigurationCheck(deleted.Checks, "configuration_resource_version_advanced") {
		t.Fatalf("private source delete lacked absence/version proof: %+v", deleted)
	}
	deletedSource, err := fixture.services.Skills.GetCatalogSourceState(fixture.ownerCtx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	importedSkill, err := fixture.services.Skills.GetCatalogSkillState(fixture.ownerCtx, skillID)
	if err != nil {
		t.Fatal(err)
	}
	if deletedSource.Exists || !importedSkill.Exists {
		t.Fatalf("source deletion affected wrong state: source=%+v skill=%+v", deletedSource, importedSkill)
	}

	changes, err := fixture.services.Configuration.ListChanges(
		fixture.ownerCtx,
		actor,
		configurationsvc.DomainSkills,
		20,
	)
	if err != nil {
		t.Fatal(err)
	}
	auditJSON, err := json.Marshal(changes)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(auditJSON, []byte(initialToken)) || bytes.Contains(auditJSON, []byte(rotatedToken)) {
		t.Fatalf("private source audit leaked a bearer token: %s", auditJSON)
	}
}

func TestPrivateSkillSourceConversationIsOwnerMainOnly(t *testing.T) {
	fixture := newScopedConfigurationFixture(t)
	worker := fixture.createAgent(t, "Private Source Boundary Worker")
	actor := configurationsvc.Actor{
		OwnerUserID: worker.OwnerUserID,
		AgentID:     worker.AgentID,
		SessionKey:  "agent:" + worker.AgentID + ":ws:dm:private-source-boundary",
		ContextKind: configurationsvc.ContextKindAgent,
		ContextID:   worker.AgentID,
	}
	bindConfigurationTestRound(t, fixture.services, &actor)
	_, err := fixture.services.Configuration.PlanChange(
		fixture.ownerCtx,
		actor,
		configurationsvc.ChangeRequest{
			Domain:    configurationsvc.DomainSkills,
			Operation: "create_private_source",
			Input: json.RawMessage(`{
				"name":"Forbidden",
				"url":"https://skills.example.com",
				"auth_type":"none"
			}`),
		},
	)
	if err == nil || (!strings.Contains(err.Error(), "无权") && !strings.Contains(err.Error(), "不支持")) {
		t.Fatalf("ordinary Agent private source error = %v, want owner-only denial", err)
	}
}

func privateSkillArchive(t *testing.T, name string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create(name + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write([]byte("---\nname: " + name + "\ntitle: Conversation Private Skill\ndescription: private source conversation test\n---\n\n# Conversation Private Skill\n")); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func privateSourceID(
	t *testing.T,
	service *skillsvc.Service,
	ctx context.Context,
) string {
	t.Helper()
	sources, err := service.ListExternalSkillSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if source.Deletable {
			return source.SourceID
		}
	}
	t.Fatal("private source not found")
	return ""
}
