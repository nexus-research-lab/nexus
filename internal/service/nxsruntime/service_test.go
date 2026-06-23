package nxsruntime

import (
	"testing"

	bridgenxs "github.com/nexus-research-lab/nexus-agent-sdk-bridge/runtimes/nxs"
)

func TestStatusMapsBridgeRuntimeStatus(t *testing.T) {
	service := NewService()
	service.inspector = func() runtimeInspector {
		return fakeRuntimeInspector{
			status: bridgenxs.Status{
				Available:   true,
				Path:        "/tmp/nxs",
				Source:      bridgenxs.RuntimeSourceEnv,
				CanDownload: false,
			},
		}
	}

	status := service.Status()
	if !status.Available || status.Path != "/tmp/nxs" || status.Source != "env" || status.Message != "" {
		t.Fatalf("Status() = %+v, want mapped bridge runtime", status)
	}
}

func TestStatusAddsProductMessageForMissingRuntime(t *testing.T) {
	service := NewService()
	service.inspector = func() runtimeInspector {
		return fakeRuntimeInspector{
			status: bridgenxs.Status{
				Error: bridgenxs.StatusErrorNotFound,
			},
		}
	}

	status := service.Status()
	if status.Available || status.CanDownload || status.Message == "" {
		t.Fatalf("Status() = %+v, want explicit-path missing runtime message", status)
	}
}

type fakeRuntimeInspector struct {
	status bridgenxs.Status
}

func (i fakeRuntimeInspector) Status() bridgenxs.Status {
	return i.status
}
