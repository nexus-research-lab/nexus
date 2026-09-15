package nxsruntime

import (
	"context"
	"errors"
	bridgenxs "github.com/nexus-research-lab/nexus-agent-sdk-bridge/runtimes/nxs"
	"os"
	"testing"
)

func TestSandboxDiagnosisDoesNotInventAvailability(t *testing.T) {
	tests := []struct {
		diagnosis *bridgenxs.SandboxBackendStatus
		err       error
		state     string
	}{
		{nil, nil, "unknown"},
		{&bridgenxs.SandboxBackendStatus{BackendSupported: true, DependenciesAvailable: true}, errors.New("probe failed"), "unknown"},
		{&bridgenxs.SandboxBackendStatus{Platform: "windows"}, nil, "unsupported"},
		{&bridgenxs.SandboxBackendStatus{BackendSupported: true}, nil, "missing_dependencies"},
		{&bridgenxs.SandboxBackendStatus{BackendSupported: true, DependenciesAvailable: true}, nil, "dependencies_available"},
	}
	for _, test := range tests {
		if got := projectSandboxStatus(test.diagnosis, test.err); got.State != test.state {
			t.Fatalf("got %+v want %s", got, test.state)
		}
	}
}

func TestSandboxDiagnosisRealNXS(t *testing.T) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("set NEXUS_SANDBOX_TEST_BINARY for native integration")
	}
	t.Setenv("NEXUS_NXS_COMMAND_PATH", binary)
	before := Status()
	if !before.Available || before.Sandbox != nil {
		t.Fatalf("ordinary file check changed: %+v", before)
	}
	got := StatusWithSandbox(context.Background())
	if !got.Available || got.Sandbox == nil || got.Sandbox.State == "unknown" {
		t.Fatalf("real diagnosis failed: %+v", got)
	}
	if got.Path != before.Path {
		t.Fatal("diagnostic replaced selected runtime")
	}
	t.Logf("sandbox diagnosis: %+v", got.Sandbox)
}
