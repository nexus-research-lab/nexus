// INPUT: Goal/Execution CLI domain、目录或精确 operation 与固定 inspect operation。
// OUTPUT: 不复制 schema 的自描述 contract/inspect/invoke 命令模板断言。
// POS: 防止 Skill 不可读时 Agent 再次靠 shell 探测语义命令协议。
package cli

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
)

func TestRuntimeEntrypointArgsRequiresThePrivateHostMarker(t *testing.T) {
	arguments, ok := RuntimeEntrypointArgs([]string{
		protocol.NexusCommandHostEntrypointArgument,
		"--json",
		"goal",
		"inspect",
	})
	if !ok || strings.Join(arguments, " ") != "--json goal inspect" {
		t.Fatalf("runtime host entrypoint = %#v, %t", arguments, ok)
	}
	if arguments, ok = RuntimeEntrypointArgs([]string{"--json", "goal", "inspect"}); ok || arguments != nil {
		t.Fatalf("ordinary host arguments entered runtime CLI = %#v, %t", arguments, ok)
	}
}

func TestRuntimeSemanticContractCommandUsageMakesExactContractMandatory(t *testing.T) {
	directory := runtimeSemanticContractCommandUsage("execution", "", "get_execution")
	if !strings.Contains(directory["next"], "exact operation contract") ||
		directory["operation_contract"] != `"${NEXUS_COMMAND_PATH}" --json execution contract --operation '<operation>'` ||
		directory["inspect"] != `"${NEXUS_COMMAND_PATH}" --json execution inspect` ||
		directory["inspect_explicit"] != `"${NEXUS_COMMAND_PATH}" --json execution inspect --execution-id '<execution-id>'` {
		t.Fatalf("directory usage = %#v", directory)
	}

	exact := runtimeSemanticContractCommandUsage(
		"execution",
		"prepare_plan_execution",
		"get_execution",
	)
	if !strings.Contains(exact["next"], "input_staging.path") ||
		!strings.Contains(exact["next"], "Read") ||
		!strings.Contains(exact["next"], "every new mutation input write") ||
		!strings.Contains(exact["next"], "never reuse a remembered path") ||
		!strings.Contains(exact["invoke"], "--operation 'prepare_plan_execution'") ||
		strings.Contains(exact["invoke"], "--input") {
		t.Fatalf("exact usage = %#v", exact)
	}

	inspect := runtimeSemanticContractCommandUsage("execution", "get_execution", "get_execution")
	if _, exists := inspect["invoke"]; exists || !strings.Contains(inspect["next"], "not invokable") {
		t.Fatalf("inspect usage = %#v", inspect)
	}
}

func TestRuntimeSemanticInvokeHasOneHostManagedInputPath(t *testing.T) {
	command := newRuntimeSemanticInvokeCommand("execution")
	if command.Flags().Lookup("input") != nil || command.Flags().Lookup("input-file") != nil {
		t.Fatal("Goal/Execution invoke must not expose parallel inline or caller-selected file transports")
	}
}

func TestRuntimeSemanticInspectKeepsOnlyTheExecutionHistoryLocator(t *testing.T) {
	execution := newRuntimeSemanticInspectCommand("execution")
	if execution.Flags().Lookup("execution-id") == nil ||
		execution.Flags().Lookup("input") != nil || execution.Flags().Lookup("input-file") != nil {
		t.Fatal("execution inspect must expose only its non-authorizing history locator")
	}
	goal := newRuntimeSemanticInspectCommand("goal")
	if goal.Flags().Lookup("execution-id") != nil {
		t.Fatal("goal inspect must remain the zero-input current Goal read")
	}
}

func TestRuntimeSemanticResultEnvelopeIsFlatAndAlwaysTyped(t *testing.T) {
	payload := runtimeSemanticResultEnvelope(
		"goal",
		runtimecommand.ActionInvoke,
		"audit_objective_alignment",
		"goal-audit-1",
		runtimecommand.Result{
			Content: []map[string]any{{"type": "text", "text": "legacy mirror"}},
			StructuredContent: map[string]any{
				"outcome": "applied",
			},
		},
	)
	if payload["is_error"] != false || payload["operation"] != "audit_objective_alignment" ||
		payload["request_id"] != "goal-audit-1" ||
		payload["data"].(map[string]any)["outcome"] != "applied" {
		t.Fatalf("semantic envelope = %#v", payload)
	}
	_, hasResult := payload["result"]
	_, hasContent := payload["content"]
	if hasResult || hasContent {
		t.Fatalf("CLI leaked internal MCP-shaped result mirror: %#v", payload)
	}

	errorPayload := runtimeSemanticResultEnvelope(
		"execution",
		runtimecommand.ActionInspect,
		"",
		"",
		runtimecommand.Result{IsError: true},
	)
	if errorPayload["is_error"] != true || errorPayload["data"] == nil ||
		len(errorPayload["data"].(map[string]any)) != 0 {
		t.Fatalf("empty semantic error envelope = %#v", errorPayload)
	}
}
