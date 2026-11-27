// INPUT: Deterministic synthetic histories in temporary SQLite/isolated PostgreSQL.
// OUTPUT: Warm read latency, Go allocations, returned facts/JSON size, full/window projection parity and public skeleton/node-detail reads.
// POS: WorkGraph read-path benchmarks and cross-driver regression; excludes providers and installed state.
package storage_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationservice "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/storage"
	orchestrationrepo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func BenchmarkWorkGraphRuntimeRead(b *testing.B) {
	for _, driver := range []string{"sqlite", "postgres"} {
		b.Run(driver, func(b *testing.B) {
			for _, profile := range []string{"detail", "mixed"} {
				b.Run(profile, func(b *testing.B) {
					for _, size := range []int{1000, 10000, 50000} {
						b.Run(fmt.Sprint(size), func(b *testing.B) {
							cfg := integrationDatabase(b, driver)
							migrateIntegrationDatabase(b, cfg, 0)
							db := openIntegrationDatabase(b, cfg)
							repo := orchestrationrepo.NewSQLRepository(driver, db)
							e := seedWorkGraphReadHistory(b, db, repo, driver, profile, size)
							var full protocol.ExecutionRuntimeGraph
							for _, mode := range []string{"full", "window"} {
								b.Run(mode, func(b *testing.B) {
									read := func() protocol.ExecutionRuntimeGraph {
										// A broken query must fail explicitly, not stall a large benchmark indefinitely.
										ctx, cancel := context.WithTimeout(b.Context(), 45*time.Second)
										defer cancel()
										graph, err := readWorkGraphHistory(ctx, repo, e, mode)
										if err != nil {
											b.Fatal(err)
										}
										return graph
									}
									warm := read()
									if mode == "full" {
										full = warm
									} else if len(full.Nodes) > 0 {
										assertWorkGraphReadParity(b, repo, e, full, warm)
									}
									encoded, err := json.Marshal(warm)
									if err != nil {
										b.Fatal(err)
									}
									b.ReportAllocs()
									b.ResetTimer()
									for range b.N {
										graph := read()
										if len(graph.Nodes) != len(warm.Nodes) || len(graph.Edges) != len(warm.Edges) {
											b.Fatal("unstable read")
										}
									}
									b.StopTimer()
									b.ReportMetric(float64(len(warm.Nodes)), "nodes/op")
									b.ReportMetric(float64(len(warm.Edges)), "edges/op")
									b.ReportMetric(float64(len(encoded)), "json-B/op")
								})
							}
						})
					}
				})
			}
		})
	}
}

// BenchmarkWorkGraphPublicRead 度量 HTTP 读取面：cold 是失效后的首次投影，warm 是
// 同一失效窗口内的重复读取（多页面/多次刷新共享缓存）。json-B/op 是骨架体积，
// full-json-B/op 是改造前 latest 返回的完整投影体积。
func BenchmarkWorkGraphPublicRead(b *testing.B) {
	for _, profile := range []string{"detail", "mixed"} {
		b.Run(profile, func(b *testing.B) {
			for _, size := range []int{1000, 10000} {
				b.Run(fmt.Sprint(size), func(b *testing.B) {
					cfg := integrationDatabase(b, "sqlite")
					migrateIntegrationDatabase(b, cfg, 0)
					db := openIntegrationDatabase(b, cfg)
					repo := orchestrationrepo.NewSQLRepository("sqlite", db)
					e := seedWorkGraphReadHistory(b, db, repo, "sqlite", profile, size)
					full, err := orchestrationservice.NewService(repo).GetView(b.Context(), e.OwnerUserID, e.SessionKey, e.ID)
					if err != nil || full == nil {
						b.Fatal(full, err)
					}
					fullJSON, _ := json.Marshal(full)
					for _, mode := range []string{"cold", "warm"} {
						b.Run(mode, func(b *testing.B) {
							shared := orchestrationservice.NewService(repo)
							read := func(svc *orchestrationservice.Service) *protocol.ExecutionView {
								view, err := svc.GetLatestSkeletonView(b.Context(), e.OwnerUserID, e.SessionKey)
								if err != nil || view == nil {
									b.Fatal(view, err)
								}
								return view
							}
							encoded, _ := json.Marshal(read(shared))
							b.ReportAllocs()
							b.ResetTimer()
							for range b.N {
								svc := shared
								if mode == "cold" {
									svc = orchestrationservice.NewService(repo)
								}
								if _, err := json.Marshal(read(svc)); err != nil {
									b.Fatal(err)
								}
							}
							b.StopTimer()
							b.ReportMetric(float64(len(encoded)), "json-B/op")
							b.ReportMetric(float64(len(fullJSON)), "full-json-B/op")
						})
					}
				})
			}
		})
	}
}

func TestWorkGraphPublicReadReturnsSkeletonAndNodeDetail(t *testing.T) {
	cfg := integrationDatabase(t, "sqlite")
	migrateIntegrationDatabase(t, cfg, 0)
	db := openIntegrationDatabase(t, cfg)
	repo := orchestrationrepo.NewSQLRepository("sqlite", db)
	e := seedWorkGraphReadHistory(t, db, repo, "sqlite", "mixed", 1000)
	svc := orchestrationservice.NewService(repo)
	full, err := svc.GetView(t.Context(), e.OwnerUserID, e.SessionKey, e.ID)
	if err != nil || full == nil {
		t.Fatal(full, err)
	}
	skeleton, err := svc.GetLatestSkeletonView(t.Context(), e.OwnerUserID, e.SessionKey)
	if err != nil || skeleton == nil {
		t.Fatal(skeleton, err)
	}
	fullJSON, _ := json.Marshal(full)
	skeletonJSON, _ := json.Marshal(skeleton)
	// 骨架体积不随 NodeRun 历史与摘要增长；固定夹具上应远小于完整投影。
	if len(skeletonJSON)*3 > len(fullJSON) {
		t.Fatalf("skeleton bytes = %d, full bytes = %d", len(skeletonJSON), len(fullJSON))
	}
	visible := make(map[string]bool, len(skeleton.Graph.Nodes))
	for _, node := range skeleton.Graph.Nodes {
		if node.Visibility == protocol.ExecutionGraphNodeDetail || len(node.Runs) > 0 || node.ResultSummary != "" || node.ErrorSummary != "" {
			t.Fatalf("skeleton leaked run facts: %+v", node)
		}
		visible[node.ID] = true
	}
	for _, edge := range skeleton.Graph.Edges {
		if !visible[edge.SourceNodeID] || !visible[edge.TargetNodeID] {
			t.Fatalf("skeleton edge references a dropped node: %+v", edge)
		}
	}
	if skeleton.Graph.RuntimeNodeTotal != full.Graph.RuntimeNodeTotal || len(skeleton.WorkItems) != len(full.WorkItems) {
		t.Fatalf("skeleton totals/work items diverged from full projection")
	}

	parentID, detailChildren := "", 0
	for _, node := range full.Graph.Nodes {
		if node.Visibility == protocol.ExecutionGraphNodeDetail && node.ParentNodeID != "" {
			parentID = node.ParentNodeID
			break
		}
	}
	for _, node := range full.Graph.Nodes {
		if parentID != "" && node.ParentNodeID == parentID {
			detailChildren++
		}
	}
	if parentID == "" || !visible[parentID] {
		t.Fatalf("fixture has no visible parent with detail children: %q", parentID)
	}
	detail, err := svc.GetGraphNodeDetail(t.Context(), e.OwnerUserID, e.SessionKey, e.ID, parentID)
	if err != nil || detail == nil {
		t.Fatal(detail, err)
	}
	if detail.Node.ID != parentID || len(detail.Children) != detailChildren {
		t.Fatalf("node detail = %s with %d children, want %d", detail.Node.ID, len(detail.Children), detailChildren)
	}
	if _, err := svc.GetGraphNodeDetail(t.Context(), e.OwnerUserID, "other-session", e.ID, parentID); err == nil {
		t.Fatal("node detail crossed session scope")
	}
	missing, err := svc.GetGraphNodeDetail(t.Context(), e.OwnerUserID, e.SessionKey, e.ID, "missing-node")
	if err != nil || missing != nil {
		t.Fatalf("missing node = %+v, %v", missing, err)
	}
}

func TestWorkGraphReadProjectionIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			for _, profile := range []string{"detail", "mixed"} {
				t.Run(profile, func(t *testing.T) {
					cfg := integrationDatabase(t, driver)
					migrateIntegrationDatabase(t, cfg, 161)
					db := openIntegrationDatabase(t, cfg)
					repo := orchestrationrepo.NewSQLRepository(driver, db)
					e := seedWorkGraphReadHistory(t, db, repo, driver, profile, 1000)
					// A legacy invoke edge may name a pruned source. The view uses
					// the target's exact parent, but must preserve durable edge identity.
					if _, err := db.ExecContext(t.Context(), `UPDATE runtime_graph_edge_runs SET source_node_run_id = 'tool-000019' WHERE edge_run_id = 'invoke-tool-000021'`); err != nil {
						t.Fatal(err)
					}
					// The last two successes share a timestamp; the run ID must
					// deterministically retain tool-000099 on both database engines.
					if _, err := db.ExecContext(t.Context(), `UPDATE runtime_graph_node_runs SET started_at = (SELECT started_at FROM runtime_graph_node_runs WHERE node_run_id = 'tool-000098') WHERE node_run_id = 'tool-000099'`); err != nil {
						t.Fatal(err)
					}
					seedForeignWorkGraphEdges(t, repo, e)
					outside := integrationRuntimeNode(e, "outside-loop-target", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
					outside.ExecutionID = "other-execution"
					if err := repo.UpsertRuntimeGraphNode(t.Context(), outside); err != nil {
						t.Fatal(err)
					}
					// A loop target can be outside this read scope; service can resolve
					// its destination from the source's exact Work Item segment.
					if err := repo.UpsertRuntimeGraphEdge(t.Context(), protocol.ExecutionRuntimeEdgeRun{
						ID: "loop-outside-target", GraphID: outside.GraphID, OwnerUserID: e.OwnerUserID, SessionKey: e.SessionKey,
						SourceNodeID: "tool-000030", TargetNodeID: outside.ID, Kind: protocol.ExecutionRuntimeEdgeLoopBack, CreatedAt: outside.StartedAt,
					}); err != nil {
						t.Fatal(err)
					}
					full, err := readWorkGraphHistory(t.Context(), repo, e, "full")
					if err != nil {
						t.Fatal(err)
					}
					if full.NodeTotal != 1010 || full.EdgeTotal != 1021 {
						t.Fatalf("foreign facts entered scope: nodes=%d edges=%d", full.NodeTotal, full.EdgeTotal)
					}
					if err := db.Close(); err != nil {
						t.Fatal(err)
					}
					// Build the new indexes over populated 161 data, not only empty tables.
					migrateIntegrationDatabase(t, cfg, 0)
					db = openIntegrationDatabase(t, cfg)
					repo = orchestrationrepo.NewSQLRepository(driver, db)
					window, err := readWorkGraphHistory(t.Context(), repo, e, "window")
					if err != nil {
						t.Fatal(err)
					}
					assertWorkGraphReadParity(t, repo, e, full, window)
				})
			}
		})
	}
}

func seedForeignWorkGraphEdges(t *testing.T, repo *orchestrationrepo.Repository, e protocol.Execution) {
	t.Helper()
	for i, scope := range []string{"owner", "session", "graph"} {
		edge := protocol.ExecutionRuntimeEdgeRun{
			ID: "foreign-" + scope, GraphID: "execution:" + e.ID, OwnerUserID: e.OwnerUserID, SessionKey: e.SessionKey,
			SourceNodeID: fmt.Sprintf("root-%06d", i*100), TargetNodeID: "tool-000021", Kind: protocol.ExecutionRuntimeEdgeGuard,
			CreatedAt: time.Date(2026, 1, 1, 0, 0, 22, 0, time.UTC),
		}
		switch scope {
		case "owner":
			edge.OwnerUserID = "other-owner"
		case "session":
			edge.SessionKey = "other-session"
		case "graph":
			edge.GraphID = "other-graph"
		}
		if err := repo.UpsertRuntimeGraphEdge(t.Context(), edge); err != nil {
			t.Fatal(err)
		}
	}
}

func readWorkGraphHistory(ctx context.Context, repo *orchestrationrepo.Repository, e protocol.Execution, mode string) (protocol.ExecutionRuntimeGraph, error) {
	if mode == "full" {
		return repo.RuntimeGraphForMigration(ctx, e)
	}
	return repo.GetWorkGraphRuntimeGraph(ctx, e.OwnerUserID, e.SessionKey, e.ID, e.RootRoundID)
}

// Every 100 tool records share one exact agent round. Both profiles contain
// failures/recoveries, late artifacts, retry/loop edges and a detail parent;
// mixed additionally has 20% canvas actions and 5% failed tools. All payloads
// are synthetic. Inserts are prepared in one transaction, outside read timing.
func seedWorkGraphReadHistory(t testing.TB, db *sql.DB, repo *orchestrationrepo.Repository, driver, profile string, size int) protocol.Execution {
	t.Helper()
	batch := prepareIntegrationBatch(t, legacyPlanWriter{repo, db, driver}, "read-history")
	e, err := repo.Get(t.Context(), batch.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	dialect := storage.NewSQLDialect(driver)
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	binds := strings.Split(dialect.BindList(27), ",")
	binds[24] = dialect.JSONValue(25)
	nodes, err := tx.PrepareContext(t.Context(), `INSERT INTO runtime_graph_node_runs (
node_run_id, graph_id, owner_user_id, session_key, execution_id, node_kind,
subject_id, parent_subject_id, root_round_id, runtime_round_id, agent_round_id,
agent_id, name, description, status, failed, result_summary, error_code, error_summary,
summary_truncated, duration_ms, started_at, updated_at, finished_at, metadata_json,
workgraph_read_class, recovery_key) VALUES (`+strings.Join(binds, ",")+`)`)
	if err != nil {
		t.Fatal(err)
	}
	defer nodes.Close()
	edges, err := tx.PrepareContext(t.Context(), `INSERT INTO runtime_graph_edge_runs (
edge_run_id, graph_id, owner_user_id, session_key, source_node_run_id, target_node_run_id, edge_kind, created_at)
VALUES (`+dialect.BindList(8)+`)`)
	if err != nil {
		t.Fatal(err)
	}
	defer edges.Close()
	insertNode := func(n protocol.ExecutionRuntimeNodeRun) {
		n = orchestrationservice.PrepareRuntimeGraphRead(n)
		metadata, err := json.Marshal(n.Metadata)
		if err != nil {
			t.Fatal(err)
		}
		_, err = nodes.ExecContext(t.Context(), n.ID, n.GraphID, n.OwnerUserID, n.SessionKey, n.ExecutionID,
			n.Kind, n.SubjectID, n.ParentSubjectID, n.RootRoundID, n.RuntimeRoundID, n.AgentRoundID,
			n.AgentID, n.Name, n.Description, n.Status, n.Failed, n.ResultSummary, n.ErrorCode, n.ErrorSummary,
			false, 1, dialect.TimestampValue(n.StartedAt), dialect.TimestampValue(n.UpdatedAt), dialect.TimestampValue(n.UpdatedAt),
			string(metadata), n.WorkGraphReadClass, n.RecoveryKey)
		if err != nil {
			t.Fatal(err)
		}
	}
	insertEdge := func(source, target protocol.ExecutionRuntimeNodeRun, kind protocol.ExecutionRuntimeEdgeKind) {
		_, err := edges.ExecContext(t.Context(), string(kind)+"-"+target.ID, target.GraphID, e.OwnerUserID, e.SessionKey,
			source.ID, target.ID, kind, dialect.TimestampValue(target.StartedAt))
		if err != nil {
			t.Fatal(err)
		}
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var artifacts []protocol.ExecutionRuntimeNodeRun
	for offset := 0; offset < size; offset += 100 {
		root := integrationRuntimeNode(*e, fmt.Sprintf("root-%06d", offset), start.Add(time.Duration(offset)*time.Second))
		root.Kind, root.Name = protocol.ExecutionRuntimeNodeAgent, "lead"
		root.AgentRoundID = "agent-round-" + root.ID
		root.Metadata = map[string]any{"execution_lane": "coordination"}
		insertNode(root)
		var prior, parent protocol.ExecutionRuntimeNodeRun
		for i := offset; i < min(offset+100, size); i++ {
			n := integrationRuntimeNode(*e, fmt.Sprintf("tool-%06d", i), start.Add(time.Duration(i)*time.Second+time.Millisecond))
			n.AgentRoundID, n.ParentSubjectID, n.Name = root.AgentRoundID, root.SubjectID, "Read"
			n.Description = "Read synthetic workspace evidence"
			n.ResultSummary = strings.Repeat("synthetic evidence ", 24)
			n.Metadata = map[string]any{"fixture": true}
			local := i - offset
			if profile == "mixed" && local%5 == 0 {
				n.Name = "WebSearch"
			}
			if local == 0 || profile == "mixed" && local%20 == 1 {
				n.Status, n.Failed, n.ErrorCode, n.ErrorSummary = protocol.ExecutionRuntimeNodeFailed, true, "fixture_failure", "Synthetic failure"
			}
			if local == 21 {
				n.ParentSubjectID, n.Name = parent.SubjectID, "WebSearch"
			}
			insertNode(n)
			if local == 21 {
				insertEdge(parent, n, protocol.ExecutionRuntimeEdgeInvoke)
			} else {
				insertEdge(root, n, protocol.ExecutionRuntimeEdgeInvoke)
			}
			if local == 11 {
				insertEdge(prior, n, protocol.ExecutionRuntimeEdgeRetry)
			}
			if local == 31 {
				insertEdge(prior, n, protocol.ExecutionRuntimeEdgeLoopBack)
			}
			if local == 12 {
				artifacts = append(artifacts, n)
			}
			if local == 20 {
				parent = n
			}
			prior = n
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, n := range artifacts {
		if err := repo.UpsertRuntimeGraphArtifact(t.Context(), protocol.ExecutionRuntimeArtifactRef{
			ID: "artifact-" + n.ID, GraphID: n.GraphID, OwnerUserID: n.OwnerUserID, SessionKey: n.SessionKey,
			ExecutionID: e.ID, RootRoundID: n.RootRoundID, AgentRoundID: n.AgentRoundID, ToolUseID: n.SubjectID,
			Artifact:  protocol.WorkspaceFileArtifactBlock{ID: "artifact-" + n.ID, Path: "output/" + n.ID + ".txt", SourceToolUseID: n.SubjectID},
			CreatedAt: n.StartedAt, UpdatedAt: n.UpdatedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"runtime_graph_node_runs", "runtime_graph_edge_runs", "runtime_graph_artifact_refs"} {
		if _, err := db.ExecContext(t.Context(), "ANALYZE "+table); err != nil {
			t.Fatal(err)
		}
	}
	return *e
}

type workGraphReadFixtureRepository struct {
	*orchestrationrepo.Repository
	graph protocol.ExecutionRuntimeGraph
}

func (r workGraphReadFixtureRepository) GetWorkGraphRuntimeGraph(context.Context, string, string, string, string) (protocol.ExecutionRuntimeGraph, error) {
	return r.graph, nil
}

func assertWorkGraphReadParity(t testing.TB, repo *orchestrationrepo.Repository, e protocol.Execution, full, window protocol.ExecutionRuntimeGraph) {
	t.Helper()
	if full.NodeTotal != window.NodeTotal || full.EdgeTotal != window.EdgeTotal || len(window.Nodes) >= len(full.Nodes) || window.NodesTruncated || window.EdgesTruncated {
		t.Fatalf("window totals/retention: full=%d/%d window=%d/%d totals=%d/%d", len(full.Nodes), len(full.Edges), len(window.Nodes), len(window.Edges), window.NodeTotal, window.EdgeTotal)
	}
	edgeIDs := make(map[string]bool, len(window.Edges))
	for _, edge := range window.Edges {
		edgeIDs[edge.ID] = true
	}
	for _, edge := range full.Edges {
		if (edge.Kind == protocol.ExecutionRuntimeEdgeRetry || edge.Kind == protocol.ExecutionRuntimeEdgeLoopBack) && !edgeIDs[edge.ID] {
			t.Fatalf("window lost control edge: %+v", edge)
		}
	}
	project := func(graph protocol.ExecutionRuntimeGraph) protocol.ExecutionGraphView {
		svc := orchestrationservice.NewService(workGraphReadFixtureRepository{repo, graph})
		view, err := svc.GetView(t.Context(), e.OwnerUserID, e.SessionKey, e.ID)
		if err != nil || view == nil {
			t.Fatalf("project fixture: %v, %v", view, err)
		}
		if view.Graph.RuntimeNodeTotal == 0 || view.Graph.RuntimeEdgeTotal == 0 {
			t.Fatal("fixture has no visible runtime facts")
		}
		// Position is the pre-window source ordinal; compare retained ordering
		// and all facts while ignoring gaps left by discarded source rows.
		slices.SortStableFunc(view.Graph.Nodes, func(a, b protocol.ExecutionGraphNodeView) int { return a.Position - b.Position })
		for i := range view.Graph.Nodes {
			view.Graph.Nodes[i].Position = i
		}
		slices.SortFunc(view.Graph.Edges, func(a, b protocol.ExecutionGraphEdgeView) int { return strings.Compare(a.ID, b.ID) })
		return view.Graph
	}
	want, got := project(full), project(window)
	if len(want.Nodes) != len(got.Nodes) || len(want.Edges) != len(got.Edges) {
		t.Fatalf("projected facts: full=%d/%d window=%d/%d", len(want.Nodes), len(want.Edges), len(got.Nodes), len(got.Edges))
	}
	for i := range want.Nodes {
		if !reflect.DeepEqual(want.Nodes[i], got.Nodes[i]) {
			t.Fatalf("projected node %d: full=%+v window=%+v", i, want.Nodes[i], got.Nodes[i])
		}
	}
	for i := range want.Edges {
		if !reflect.DeepEqual(want.Edges[i], got.Edges[i]) {
			t.Fatalf("projected edge %d: full=%+v window=%+v", i, want.Edges[i], got.Edges[i])
		}
	}
	want.Nodes, got.Nodes, want.Edges, got.Edges = nil, nil, nil, nil
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("projected totals/flags: full=%+v window=%+v", want, got)
	}
}
