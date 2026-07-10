package cli

import (
	"strings"
	"testing"
)

func TestMemoryCommandRemoved(t *testing.T) {
	cfg := newCLITestConfig(t)
	errText := runCLICommandError(t, cfg, nil, "memory")
	if !strings.Contains(errText, `unknown command "memory"`) {
		t.Fatalf("memory command error = %q, want command removed", errText)
	}
}
