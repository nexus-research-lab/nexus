package agent_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentpkg "github.com/nexus-research-lab/nexus/internal/service/agent"
)

func TestAgentCreationRequestReplaysExactResultAndSurvivesDeletion(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := context.Background()
	request := protocol.CreateRequest{
		Name:              "Recovery Agent",
		CreationRequestID: "web-create:recovery-agent",
	}

	created, err := service.CreateAgent(ctx, request)
	if err != nil {
		t.Fatalf("first CreateAgent() error = %v", err)
	}
	replayed, err := service.CreateAgent(ctx, request)
	if err != nil {
		t.Fatalf("replayed CreateAgent() error = %v", err)
	}
	if replayed.AgentID != created.AgentID {
		t.Fatalf("replayed agent_id = %q, want %q", replayed.AgentID, created.AgentID)
	}
	result, err := service.GetAgentCreationRequestResult(ctx, request.CreationRequestID)
	if err != nil || result.Status != protocol.AgentCreationRequestCommitted ||
		result.Agent == nil || result.Agent.AgentID != created.AgentID {
		t.Fatalf("creation result = %#v, err=%v", result, err)
	}

	conflict := request
	conflict.Name = "Different Agent"
	if _, err = service.CreateAgent(ctx, conflict); !errors.Is(err, agentpkg.ErrAgentCreationRequestConflict) {
		t.Fatalf("conflicting replay error = %v", err)
	}
	if err = service.DeleteAgent(ctx, created.AgentID); err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}
	deletedResult, err := service.GetAgentCreationRequestResult(ctx, request.CreationRequestID)
	if err != nil || deletedResult.Status != protocol.AgentCreationRequestDeleted {
		t.Fatalf("deleted creation result = %#v, err=%v", deletedResult, err)
	}
	if _, err = service.CreateAgent(ctx, request); !errors.Is(err, agentpkg.ErrAgentCreationResultDeleted) {
		t.Fatalf("late replay error = %v, want deleted tombstone", err)
	}
}

func TestAgentCreationRequestBusinessTagChangeConflicts(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := context.Background()
	request := protocol.CreateRequest{
		Name:              "Tagged Recovery Agent",
		BusinessTags:      []string{"engineering"},
		CreationRequestID: "web-create:tag-change",
	}

	if _, err := service.CreateAgent(ctx, request); err != nil {
		t.Fatalf("first CreateAgent() error = %v", err)
	}
	conflict := request
	conflict.BusinessTags = []string{"finance"}
	if _, err := service.CreateAgent(ctx, conflict); !errors.Is(err, agentpkg.ErrAgentCreationRequestConflict) {
		t.Fatalf("business tag replay error = %v, want conflict", err)
	}
}

func TestAgentCreationRequestEquivalentBusinessTagsReplayExactResult(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := context.Background()
	request := protocol.CreateRequest{
		Name:              "Normalized Tag Agent",
		BusinessTags:      []string{"Research"},
		CreationRequestID: "web-create:normalized-tags",
	}

	created, err := service.CreateAgent(ctx, request)
	if err != nil {
		t.Fatalf("first CreateAgent() error = %v", err)
	}
	equivalent := request
	equivalent.BusinessTags = []string{" Research ", "research", "", "RESEARCH"}
	replayed, err := service.CreateAgent(ctx, equivalent)
	if err != nil {
		t.Fatalf("equivalent CreateAgent() error = %v", err)
	}
	if replayed.AgentID != created.AgentID {
		t.Fatalf("replayed agent_id = %q, want %q", replayed.AgentID, created.AgentID)
	}
	if len(replayed.BusinessTags) != 1 || replayed.BusinessTags[0] != "Research" {
		t.Fatalf("replayed business_tags = %#v, want normalized tag", replayed.BusinessTags)
	}
}

func TestAgentCreationWorkspaceFailureBecomesTerminalWithoutAgent(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	blockedRoot := filepath.Join(t.TempDir(), "workspace-is-a-file")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.WorkspacePath = blockedRoot
	service, db := newAgentTestService(t, cfg)
	request := protocol.CreateRequest{
		Name:              "Blocked Workspace Agent",
		CreationRequestID: "web-create:blocked-workspace",
	}
	if _, err := service.CreateAgent(context.Background(), request); !errors.Is(err, agentpkg.ErrAgentCreationFailed) {
		t.Fatalf("CreateAgent() error = %v, want terminal creation failure", err)
	}
	result, err := service.GetAgentCreationRequestResult(context.Background(), request.CreationRequestID)
	if err != nil || result.Status != protocol.AgentCreationRequestFailed {
		t.Fatalf("creation result = %#v, err=%v", result, err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agents WHERE name = ?`, request.Name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed creation wrote %d Agent rows", count)
	}
}

func TestConcurrentAgentCreationRequestProducesOneAgentIdentity(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	ctx := context.Background()
	request := protocol.CreateRequest{
		Name:              "Concurrent Agent",
		CreationRequestID: "web-create:concurrent-agent",
	}

	const callers = 8
	ids := make(chan string, callers)
	errs := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Add(1)
		go func() {
			defer group.Done()
			item, err := service.CreateAgent(ctx, request)
			if err != nil {
				errs <- err
				return
			}
			ids <- item.AgentID
		}()
	}
	group.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent CreateAgent() error = %v", err)
	}
	unique := make(map[string]struct{})
	for id := range ids {
		unique[id] = struct{}{}
	}
	if len(unique) != 1 {
		t.Fatalf("concurrent create returned identities = %#v", unique)
	}
	items, err := service.ListAgents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range items {
		if item.Name == request.Name {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("created Agent count = %d, want 1", count)
	}
}

func TestAgentCreationRequestIdentityIsOwnerScoped(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	service, _ := newAgentTestService(t, cfg)
	request := protocol.CreateRequest{
		Name:              "Owner Scoped Agent",
		CreationRequestID: "web-create:shared-literal",
	}
	ownerA := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "owner-a"})
	ownerB := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "owner-b"})

	createdA, err := service.CreateAgent(ownerA, request)
	if err != nil {
		t.Fatalf("owner A CreateAgent() error = %v", err)
	}
	createdB, err := service.CreateAgent(ownerB, request)
	if err != nil {
		t.Fatalf("owner B CreateAgent() error = %v", err)
	}
	if createdA.AgentID == createdB.AgentID || createdA.OwnerUserID == createdB.OwnerUserID {
		t.Fatalf("owner-scoped requests collapsed: A=%#v B=%#v", createdA, createdB)
	}
	for ctx, want := range map[context.Context]string{
		ownerA: createdA.AgentID,
		ownerB: createdB.AgentID,
	} {
		result, resultErr := service.GetAgentCreationRequestResult(ctx, request.CreationRequestID)
		if resultErr != nil || result.Agent == nil || result.Agent.AgentID != want {
			t.Fatalf("owner-scoped result = %#v err=%v, want Agent %q", result, resultErr, want)
		}
	}
}

func TestConcurrentAgentCreationAcrossServicesUsesOneDurableReservation(t *testing.T) {
	cfg := newTestConfig(t)
	migrateSQLite(t, cfg.DatabaseURL)
	first, _ := newAgentTestService(t, cfg)
	second, _ := newAgentTestService(t, cfg)
	ctx := context.Background()
	if _, err := first.ListAgents(ctx); err != nil {
		t.Fatalf("initialize first service: %v", err)
	}
	if _, err := second.ListAgents(ctx); err != nil {
		t.Fatalf("initialize second service: %v", err)
	}
	request := protocol.CreateRequest{
		Name:              "Cross Instance Agent",
		CreationRequestID: "web-create:cross-instance",
	}

	start := make(chan struct{})
	results := make(chan *protocol.Agent, 2)
	errorsSeen := make(chan error, 2)
	var group sync.WaitGroup
	for _, service := range []*agentpkg.Service{first, second} {
		group.Add(1)
		go func(service *agentpkg.Service) {
			defer group.Done()
			<-start
			item, err := service.CreateAgent(ctx, request)
			if err != nil {
				errorsSeen <- err
				return
			}
			results <- item
		}(service)
	}
	close(start)
	group.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		if !errors.Is(err, agentpkg.ErrAgentCreationPending) {
			t.Fatalf("concurrent cross-service error = %v", err)
		}
	}
	var createdID string
	for item := range results {
		if createdID == "" {
			createdID = item.AgentID
		} else if item.AgentID != createdID {
			t.Fatalf("cross-service identities = %q and %q", createdID, item.AgentID)
		}
	}
	replayed, err := first.CreateAgent(ctx, request)
	if err != nil {
		t.Fatalf("final replay error = %v", err)
	}
	if createdID != "" && replayed.AgentID != createdID {
		t.Fatalf("final replay Agent = %q, want %q", replayed.AgentID, createdID)
	}
	items, err := first.ListAgents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range items {
		if item.Name == request.Name {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("cross-service created Agent count = %d, want 1", count)
	}
}
