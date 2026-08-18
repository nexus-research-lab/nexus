// INPUT: Goal/Execution CLI domain、目录或精确 operation 与固定 inspect operation。
// OUTPUT: 不复制 schema 的自描述 contract/inspect/invoke 命令模板断言。
// POS: 防止 Skill 不可读时 Agent 再次靠 shell 探测语义命令协议。
package cli

import (
	"strings"
	"testing"
)

func TestRuntimeSemanticContractCommandUsageMakesExactContractMandatory(t *testing.T) {
	directory := runtimeSemanticContractCommandUsage("execution", "", "get_execution")
	if !strings.Contains(directory["next"], "exact operation contract") ||
		directory["operation_contract"] != `"${NEXUS_COMMAND_PATH}" --json execution contract --operation '<operation>'` ||
		directory["inspect"] != `"${NEXUS_COMMAND_PATH}" --json execution inspect` {
		t.Fatalf("directory usage = %#v", directory)
	}

	exact := runtimeSemanticContractCommandUsage(
		"execution",
		"prepare_plan_execution",
		"get_execution",
	)
	if !strings.Contains(exact["next"], "input_staging.path") ||
		!strings.Contains(exact["invoke"], "--operation 'prepare_plan_execution'") ||
		!strings.Contains(exact["invoke"], `--input-file "${NEXUS_COMMAND_INPUT_PATH}"`) {
		t.Fatalf("exact usage = %#v", exact)
	}

	inspect := runtimeSemanticContractCommandUsage("execution", "get_execution", "get_execution")
	if _, exists := inspect["invoke"]; exists || !strings.Contains(inspect["next"], "not invokable") {
		t.Fatalf("inspect usage = %#v", inspect)
	}
}
