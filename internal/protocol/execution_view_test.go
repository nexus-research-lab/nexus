package protocol

import (
	"encoding/json"
	"testing"
)

func TestExecutionViewJSONKeepsRequiredGraphEnvelope(t *testing.T) {
	payload, err := json.Marshal(ExecutionView{
		ID:         "execution-1",
		SessionKey: "agent:nexus:ws:dm:session-1",
		ScopeKind:  ExecutionScopeDM,
		Graph:      ExecutionGraphView{},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["scope_kind"] != string(ExecutionScopeDM) {
		t.Fatalf("scope_kind = %#v", decoded["scope_kind"])
	}
	graph, ok := decoded["graph"].(map[string]any)
	if !ok {
		t.Fatalf("graph = %#v, want required object", decoded["graph"])
	}
	for _, key := range []string{
		"runtime_node_total",
		"runtime_edge_total",
		"runtime_nodes_truncated",
		"runtime_edges_truncated",
	} {
		if _, exists := graph[key]; !exists {
			t.Fatalf("graph missing required key %q: %#v", key, graph)
		}
	}
}
