package nxsruntime

import (
	"errors"
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
				CanDownload: true,
				Error:       bridgenxs.StatusErrorNotFound,
			},
		}
	}

	status := service.Status()
	if status.Available || !status.CanDownload || status.Message == "" {
		t.Fatalf("Status() = %+v, want downloadable missing runtime message", status)
	}
}

func TestDownloadReturnsBridgeRuntimeStatus(t *testing.T) {
	service := NewService()
	service.inspector = func() runtimeInspector {
		return fakeRuntimeInspector{
			ensureStatus: bridgenxs.Status{
				Available:   true,
				Path:        "/tmp/nxs",
				Source:      bridgenxs.RuntimeSourceCache,
				CanDownload: false,
			},
		}
	}

	status, err := service.Download()
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !status.Available || status.Path != "/tmp/nxs" || status.Source != "cache" {
		t.Fatalf("Download() = %+v, want downloaded runtime", status)
	}
}

func TestDownloadMapsBridgeFailureMessage(t *testing.T) {
	downloadErr := errors.New("manifest unavailable")
	service := NewService()
	service.inspector = func() runtimeInspector {
		return fakeRuntimeInspector{
			ensureStatus: bridgenxs.Status{
				CanDownload: true,
				Error:       bridgenxs.StatusErrorDownloadFailed,
			},
			ensureErr: downloadErr,
		}
	}

	status, err := service.Download()
	if !errors.Is(err, downloadErr) {
		t.Fatalf("Download() error = %v, want %v", err, downloadErr)
	}
	if status.Message == "" {
		t.Fatalf("Download() = %+v, want failure message", status)
	}
}

type fakeRuntimeInspector struct {
	status       bridgenxs.Status
	ensureStatus bridgenxs.Status
	ensureErr    error
}

func (i fakeRuntimeInspector) Status() bridgenxs.Status {
	return i.status
}

func (i fakeRuntimeInspector) Ensure() (bridgenxs.Status, error) {
	return i.ensureStatus, i.ensureErr
}
