package configuration_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	configurationsvc "github.com/nexus-research-lab/nexus/internal/service/configuration"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

func TestConfigurationControlPlaneAppliesAndVerifiesPreferenceChange(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, "config"))
	cfg := config.Config{
		DatabaseDriver:  "sqlite",
		DatabaseURL:     filepath.Join(root, "nexus.db"),
		DefaultAgentID:  "nexus",
		WorkspacePath:   filepath.Join(root, "workspace"),
		DefaultTimezone: "Asia/Shanghai",
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	services := server.NewAppServicesWithDB(cfg, db, nil)
	if err = services.Core.Agent.EnsureReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	mainAgent, err := services.Core.Agent.GetDefaultAgent(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	actor := configurationsvc.Actor{
		OwnerUserID: mainAgent.OwnerUserID, AgentID: mainAgent.AgentID, IsMainAgent: true,
		SessionKey: "agent:nexus:dm:main",
	}
	before, err := services.Configuration.Inspect(t.Context(), actor, []string{
		configurationsvc.DomainPreferences,
		configurationsvc.DomainProviders,
		configurationsvc.DomainAgents,
		configurationsvc.DomainChannels,
		configurationsvc.DomainConnectors,
		configurationsvc.DomainSkills,
		configurationsvc.DomainHost,
	}, true)
	if err != nil {
		t.Fatalf("inspect full mutable configuration: %v", err)
	}
	preferences := before.Domains[configurationsvc.DomainPreferences]
	input := json.RawMessage(`{"agent_sdk_diagnostics_enabled":true}`)
	plan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainPreferences, Operation: "update", Input: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentRevision != preferences.Revision {
		t.Fatalf("plan revision = %q, inspect revision = %q", plan.CurrentRevision, preferences.Revision)
	}
	applied, err := services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-pref-0001", Domain: configurationsvc.DomainPreferences,
		Operation: "update", Input: input, ExpectedRevision: plan.CurrentRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.RevisionAfter == applied.RevisionBefore {
		t.Fatalf("apply result = %+v", applied)
	}
	stored, err := services.Preferences.Get(t.Context(), actor.OwnerUserID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.AgentSDKDiagnosticsEnabled {
		t.Fatal("preference change did not reach source of truth")
	}
	replayed, err := services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-pref-0001", Domain: configurationsvc.DomainPreferences,
		Operation: "update", Input: input, ExpectedRevision: plan.CurrentRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.IdempotentReplay {
		t.Fatalf("repeated request was not replayed: %+v", replayed)
	}
	changes, err := services.Configuration.ListChanges(t.Context(), actor, configurationsvc.DomainPreferences, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Status != "success" {
		t.Fatalf("audit changes = %+v", changes)
	}

	const providerSecret = "provider-secret-must-not-leak"
	providerInput := json.RawMessage(`{
		"provider":"dialog-provider",
		"preset_key":"custom",
		"api_format":"responses",
		"display_name":"Dialog Provider",
		"auth_token":"` + providerSecret + `",
		"base_url":"https://provider.example.com/v1",
		"models_path":"/models"
	}`)
	providerPlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "create", Input: providerInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	providerApplied, err := services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0001", Domain: configurationsvc.DomainProviders,
		Operation: "create", Input: providerInput, ExpectedRevision: providerPlan.CurrentRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	appliedJSON, err := json.Marshal(providerApplied)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(appliedJSON), providerSecret) {
		t.Fatalf("apply result leaked Provider secret: %s", appliedJSON)
	}
	createdProvider, err := services.Provider.Get(t.Context(), "dialog-provider")
	if err != nil {
		t.Fatal(err)
	}
	if !createdProvider.Enabled {
		t.Fatal("conversational Provider create must default enabled=true")
	}

	updateInput := json.RawMessage(`{"display_name":"Renamed Provider"}`)
	updatePlan, err := services.Configuration.PlanChange(t.Context(), actor, configurationsvc.ChangeRequest{
		Domain: configurationsvc.DomainProviders, Operation: "update",
		Target: "dialog-provider", Input: updateInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatePlan.CurrentRevision != providerApplied.RevisionAfter {
		t.Fatalf("provider revision drifted without a change: plan=%s applied=%s", updatePlan.CurrentRevision, providerApplied.RevisionAfter)
	}
	_, err = services.Configuration.ApplyChange(t.Context(), actor, configurationsvc.ChangeRequest{
		RequestID: "integration-provider-0002", Domain: configurationsvc.DomainProviders,
		Operation: "update", Target: "dialog-provider", Input: updateInput,
		ExpectedRevision: updatePlan.CurrentRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedProvider, err := services.Provider.Get(t.Context(), "dialog-provider")
	if err != nil {
		t.Fatal(err)
	}
	if !updatedProvider.Enabled || updatedProvider.DisplayName != "Renamed Provider" {
		t.Fatalf("Provider merge patch reset existing configuration: %+v", updatedProvider)
	}
	var storedToken string
	if err = db.QueryRow(`SELECT auth_token FROM provider WHERE provider = 'dialog-provider'`).Scan(&storedToken); err != nil {
		t.Fatal(err)
	}
	if storedToken != providerSecret {
		t.Fatal("Provider merge patch did not preserve omitted auth_token")
	}
	allChanges, err := services.Configuration.ListChanges(t.Context(), actor, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	auditJSON, err := json.Marshal(allChanges)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(auditJSON), providerSecret) {
		t.Fatalf("configuration audit leaked Provider secret: %s", auditJSON)
	}
}
