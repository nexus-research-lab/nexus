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
