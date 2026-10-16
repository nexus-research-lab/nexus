package computeruse

import (
	"context"
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
	binary := filepath.Join(root, "sidecar.sh")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nsleep 0.1\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
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
