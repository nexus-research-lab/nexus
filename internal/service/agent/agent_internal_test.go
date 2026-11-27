// agent_internal_test.go 覆盖 Agent 删除协调、情绪状态版本与提示词作用域单元测试。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

type deletionTestRepository struct {
	Repository
	deleteErr error
	deleted   int
}

func (r *deletionTestRepository) DeleteAgent(context.Context, string, string) error {
	r.deleted++
	return r.deleteErr
}

type deletionTestCoordinator struct {
	cleanupErr error
	beforeErr  error
}

func (c deletionTestCoordinator) CoordinateAgentDeletion(
	ctx context.Context,
	_ string,
	_ string,
	deletePersistent func(context.Context) error,
) error {
	if c.beforeErr != nil {
		return c.beforeErr
	}
	if err := deletePersistent(ctx); err != nil {
		return err
	}
	return c.cleanupErr
}

func TestDeleteAgentPersistenceMarksCommittedCleanupFailure(t *testing.T) {
	repository := &deletionTestRepository{}
	service := &Service{
		repository:          repository,
		deletionCoordinator: deletionTestCoordinator{cleanupErr: errors.New("runtime revoke failed")},
	}

	err := service.deleteAgentPersistence(context.Background(), "owner-a", "agent-a")
	if err == nil || !AgentDeletionCommitted(err) || repository.deleted != 1 {
		t.Fatalf("已提交删除的清理失败必须可识别: err=%v deleted=%d", err, repository.deleted)
	}
}

func (s *Service) deleteAgentPersistence(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) error {
	return s.deleteAgentPersistenceAtVersion(ctx, ownerUserID, agentID, nil)
}

// SetRuntimeEmotionBaseAtVersion 仅在 version 匹配时更新基础情绪。
func SetRuntimeEmotionBaseAtVersion(
	workspacePath string,
	update RuntimeEmotionBaseUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return setRuntimeEmotionBaseAtVersion(workspacePath, update, &expectedVersion)
}

// SetRuntimeEmotionContextAtVersion 仅在 version 匹配时更新指定上下文情绪。
func SetRuntimeEmotionContextAtVersion(
	workspacePath string,
	update RuntimeEmotionContextUpdate,
	expectedVersion int64,
) (RuntimeEmotionView, error) {
	return setRuntimeEmotionContextAtVersion(workspacePath, update, &expectedVersion)
}

func TestRuntimeEmotionVersionCASSerializesConcurrentWriters(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	initial := LoadRuntimeEmotionView(workspace, "dm:test", now)
	if initial.Version != 1 {
		t.Fatalf("initial emotion version = %d, want 1", initial.Version)
	}

	first, err := SetRuntimeEmotionBaseAtVersion(
		workspace,
		RuntimeEmotionBaseUpdate{
			Mood: "curious", Energy: 7, Valence: 8,
			Description: "ready to explore", Timestamp: now,
		},
		initial.Version,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 2 || first.Base.Mood != "curious" {
		t.Fatalf("first CAS result = %+v", first)
	}
	if _, err = SetRuntimeEmotionBaseAtVersion(
		workspace,
		RuntimeEmotionBaseUpdate{
			Mood: "stale", Energy: 1, Valence: 1,
			Description: "must not win", Timestamp: now,
		},
		initial.Version,
	); !errors.Is(err, ErrRuntimeEmotionVersionConflict) {
		t.Fatalf("stale emotion write error = %v", err)
	}

	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, mood := range []string{"steady", "bright"} {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, writeErr := SetRuntimeEmotionContextAtVersion(
				workspace,
				RuntimeEmotionContextUpdate{
					ContextID: "dm:test", Mood: mood,
					Valence: 7, Trigger: "concurrent update", Timestamp: now,
				},
				first.Version,
			)
			results <- writeErr
		}()
	}
	wait.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for resultErr := range results {
		switch {
		case resultErr == nil:
			successes++
		case errors.Is(resultErr, ErrRuntimeEmotionVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent emotion error: %v", resultErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("emotion concurrent results: successes=%d conflicts=%d", successes, conflicts)
	}
	current := LoadRuntimeEmotionView(workspace, "dm:test", now)
	if current.Version != 3 || current.Context == nil {
		t.Fatalf("final emotion state = %+v", current)
	}
	safe := SafeRuntimeEmotionView(current)
	payload, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}
	if safe.StatePath != "" || string(payload) == "" ||
		json.Valid(payload) && containsJSONField(payload, "state_path") {
		t.Fatalf("safe emotion view leaked state path: %s", payload)
	}
}

func containsJSONField(payload []byte, field string) bool {
	var values map[string]any
	if json.Unmarshal(payload, &values) != nil {
		return false
	}
	_, ok := values[field]
	return ok
}

// TestRuntimeScopeSectionStableAcrossEntryPoints 防止同一会话因驱动入口不同而翻转 system prompt。
func TestRuntimeScopeSectionStableAcrossEntryPoints(t *testing.T) {
	background := buildRuntimeScopeSection(context.Background(), "user_a")
	interactive := buildRuntimeScopeSection(authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:   "user_a",
		Username: "alice",
	}), "user_a")
	if background != interactive {
		t.Fatalf("scope section differs by entry point:\n%s\n---\n%s", background, interactive)
	}
}
