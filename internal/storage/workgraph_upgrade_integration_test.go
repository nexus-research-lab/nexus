// INPUT: Synthetic WorkGraph data frozen at migration 158 on SQLite/PostgreSQL.
// OUTPUT: Current migrations replay cleanly; backfill failure remains pending and retry preserves live facts.
// POS: Cross-driver upgrade regression using only disposable databases, not an installed Nexus state root.
package storage_test

import (
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationrepo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestWorkGraphUpgradeIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 158)
			oldDB := openIntegrationDatabase(t, cfg)
			oldRepo := orchestrationrepo.NewSQLRepository(driver, oldDB)
			batch := prepareIntegrationBatch(t, legacyPlanWriter{oldRepo, oldDB, driver}, "pre-upgrade")
			if err := oldDB.Close(); err != nil {
				t.Fatal(err)
			}
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			repo := orchestrationrepo.NewSQLRepository(driver, db)
			assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 2, 0, 2)
			prepareIntegrationBatch(t, repo, "post-upgrade")
			pending, err := repo.ListRuntimeHistoryBackfills(t.Context(), protocol.Execution{}, 10)
			if err != nil || len(pending) != 1 || pending[0].ID != batch.ExecutionID {
				t.Fatalf("frozen backlog must contain only pre-upgrade execution: %+v, %v", pending, err)
			}
			e := pending[0]
			if next, err := repo.ListRuntimeHistoryBackfills(t.Context(), e, 10); err != nil || len(next) != 0 {
				t.Fatalf("backlog keyset cursor: %+v, %v", next, err)
			}
			now := time.Now().UTC().Truncate(time.Microsecond)
			live := integrationRuntimeNode(e, "live-node", now.Add(time.Second))
			live.ResultSummary = "live result"
			if err := repo.UpsertRuntimeGraphNode(t.Context(), live); err != nil {
				t.Fatal(err)
			}
			historical := live
			historical.UpdatedAt = now
			historical.ResultSummary, historical.WorkGraphReadClass = "obsolete result", "detail"
			recovered := integrationRuntimeNode(e, "recovered-node", now)
			recovered.WorkGraphReadClass = "detail"
			edge := protocol.ExecutionRuntimeEdgeRun{
				ID: "recovered-edge", GraphID: live.GraphID, OwnerUserID: e.OwnerUserID, SessionKey: e.SessionKey,
				SourceNodeID: live.ID, TargetNodeID: recovered.ID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now,
			}
			badEdge := edge
			badEdge.ID, badEdge.OwnerUserID = "cross-owner-edge", "other-owner"
			graph := protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{historical, recovered}, Edges: []protocol.ExecutionRuntimeEdgeRun{edge, badEdge}}
			if err := repo.CompleteRuntimeHistoryBackfill(t.Context(), e, graph); !errors.Is(err, orchestrationrepo.ErrInvariant) {
				t.Fatalf("cross-owner backfill error = %v", err)
			}
			current, err := repo.GetWorkGraphRuntimeGraph(t.Context(), e.OwnerUserID, e.SessionKey, e.ID, e.RootRoundID)
			if err != nil || !current.HistoryPending || len(current.Nodes) != 1 || len(current.Edges) != 0 {
				t.Fatalf("failed backfill must roll back facts and marker: %+v, %v", current, err)
			}
			graph.Edges = graph.Edges[:1]
			for range 2 {
				if err := repo.CompleteRuntimeHistoryBackfill(t.Context(), e, graph); err != nil {
					t.Fatal(err)
				}
			}
			current, err = repo.GetWorkGraphRuntimeGraph(t.Context(), e.OwnerUserID, e.SessionKey, e.ID, e.RootRoundID)
			if err != nil || current.HistoryPending || len(current.Nodes) != 2 || len(current.Edges) != 1 {
				t.Fatalf("backfill retry: %+v, %v", current, err)
			}
			for _, node := range current.Nodes {
				if node.ID == live.ID && (node.ResultSummary != live.ResultSummary || !node.UpdatedAt.Equal(live.UpdatedAt) || node.WorkGraphReadClass != live.WorkGraphReadClass) {
					t.Fatalf("backfill overwrote a live observation: %+v", node)
				}
				if node.ID == recovered.ID && node.WorkGraphReadClass != "detail" {
					t.Fatalf("recovered read classification = %q", node.WorkGraphReadClass)
				}
			}
			if pending, err := repo.ListRuntimeHistoryBackfills(t.Context(), protocol.Execution{}, 10); err != nil || len(pending) != 0 {
				t.Fatalf("completed backlog = %+v, %v", pending, err)
			}
			// Responsibility and CAS data must remain usable after the history upgrade.
			if _, err := repo.AssignBatch(t.Context(), batch); err != nil {
				t.Fatal(err)
			}
			assertIntegrationBatchState(t, db, driver, repo, batch.ExecutionID, 3, 2, 5)
		})
	}
}
