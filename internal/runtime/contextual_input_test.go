package runtime

import (
	"strings"
	"testing"
)

func TestRoundRecoveryContextUsesInternalSourceEnvelope(t *testing.T) {
	rendered := renderContextualInputBlock(NewContextualInputBlock(
		ContextualInputNameRoundRecovery,
		"Recorded terminal reason: content_filtered.",
		0,
		nil,
	))
	if !strings.Contains(rendered, `<internal_context source="round_recovery">`) {
		t.Fatalf("round recovery context 缺少内部来源标签: %q", rendered)
	}
	if !strings.Contains(rendered, "Recorded terminal reason: content_filtered.") {
		t.Fatalf("round recovery context 丢失正文: %q", rendered)
	}
}

func TestExecutionContextUsesInternalSourceEnvelope(t *testing.T) {
	rendered := renderContextualInputBlock(NewContextualInputBlock(
		ContextualInputNameExecution,
		`<nexus_execution_context execution_version="4"></nexus_execution_context>`,
		0,
		nil,
	))
	if !strings.Contains(rendered, `<internal_context source="execution">`) {
		t.Fatalf("execution context 缺少内部来源标签: %q", rendered)
	}
	if !strings.Contains(rendered, `<nexus_execution_context execution_version="4">`) {
		t.Fatalf("execution context 丢失正文: %q", rendered)
	}
}
