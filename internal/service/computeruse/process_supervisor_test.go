package computeruse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestProcessSupervisorRestartsAfterChildExit(t *testing.T) {
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_CUA_TEST_EXIT_CHILD", "1")
	supervisor := NewProcessSupervisor(config.Config{
		DatabaseURL:           filepath.Join(root, "nexus.db"),
		ComputerUseBinaryPath: binary,
	})
	var factoryCalls atomic.Int32
	supervisor.factory = func(nexuscua.Config) (Client, error) {
		factoryCalls.Add(1)
		return &fakeClient{}, nil
	}

	first, err := supervisor.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && supervisor.ArtifactRoot() != "" {
		time.Sleep(10 * time.Millisecond)
	}
	if supervisor.ArtifactRoot() != "" {
		t.Fatal("supervisor did not observe the crashed sidecar")
	}
	second, err := supervisor.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("restart reused the dead sidecar client")
	}
	if factoryCalls.Load() != 2 {
		t.Fatalf("client factory calls = %d, want two epochs", factoryCalls.Load())
	}
	_ = supervisor.Stop(context.Background())
}

func TestMain(m *testing.M) {
	if os.Getenv("NEXUS_CUA_TEST_EXIT_CHILD") == "1" && len(os.Args) > 1 && os.Args[1] == "serve" {
		time.Sleep(100 * time.Millisecond)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

type versionedClient struct {
	fakeClient
	runtimeVersion string
}

func (c *versionedClient) GetCapabilities(ctx context.Context, timeout time.Duration) (nexuscua.DriverCapabilities, error) {
	capabilities, err := c.fakeClient.GetCapabilities(ctx, timeout)
	capabilities.RuntimeVersion = c.runtimeVersion
	return capabilities, err
}

func TestReadinessRejectsUnpinnedRuntimeVersion(t *testing.T) {
	client := &versionedClient{runtimeVersion: "0.1.0"}
	started := time.Now()
	if err := waitUntilReady(context.Background(), client, pinnedCUAVersion); !errors.Is(err, ErrRuntimeVersionMismatch) {
		t.Fatalf("waitUntilReady error = %v, want runtime version mismatch", err)
	}
	if time.Since(started) > readinessTimeout/2 {
		t.Fatal("version mismatch waited for the readiness timeout instead of failing fast")
	}
	if err := waitUntilReady(context.Background(), client, ""); err != nil {
		t.Fatalf("unpinned development readiness failed: %v", err)
	}
	client.runtimeVersion = pinnedCUAVersion
	if err := waitUntilReady(context.Background(), client, pinnedCUAVersion); err != nil {
		t.Fatalf("pinned runtime readiness failed: %v", err)
	}
}
