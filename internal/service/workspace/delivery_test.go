package workspace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestValidateDeliverablePaths(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "report.pptx"), []byte("script output"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := confinedfs.Open(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	got, err := validateDeliverablePaths(context.Background(), root, workspace, []string{"./report.pptx", filepath.Join(workspace, "report.pptx")})
	if err != nil || !reflect.DeepEqual(got, []string{"report.pptx"}) {
		t.Fatalf("canonical delivery: %v %v", got, err)
	}
	for _, path := range []string{"missing.pdf", "../other/report.pptx", filepath.Join(t.TempDir(), "report.pptx"), ".", ".agents/private.md"} {
		t.Run(path, func(t *testing.T) {
			got, err := validateDeliverablePaths(context.Background(), root, workspace, []string{"report.pptx", path})
			if err == nil || got != nil {
				t.Fatalf("partial/invalid batch accepted: %v %v", got, err)
			}
		})
	}
	if err := os.Symlink(filepath.Join(workspace, "report.pptx"), filepath.Join(workspace, "alias.pptx")); err != nil {
		t.Skip(err)
	}
	if _, err := validateDeliverablePaths(context.Background(), root, workspace, []string{"alias.pptx"}); err == nil {
		t.Fatal("accepted symbolic-link alias")
	}
}

func TestDeliverableServiceEnforcesAgentOwner(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	svc := NewService(cfg, agents)
	ctx := context.Background()
	agent, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Delivery producer"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agent.WorkspacePath, "report.pdf"), []byte("completed script output"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateDeliverables(ctx, agent.AgentID, []string{"report.pdf"}); err != nil {
		t.Fatal(err)
	}
	other := authctx.WithPrincipal(ctx, &authctx.Principal{UserID: "other-owner", Role: authctx.RoleOwner})
	if _, err := svc.ValidateDeliverables(other, agent.AgentID, []string{"report.pdf"}); err == nil {
		t.Fatal("cross-owner delivery accepted")
	}
}
