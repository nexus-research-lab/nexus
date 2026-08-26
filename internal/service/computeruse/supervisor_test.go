// INPUT: native test helper processes and deterministic typed readiness clients.
// OUTPUT: evidence for ready, epoch restart, early exit, timeout, shutdown, and bounded logs.
// POS: sidecar lifecycle regression tests without desktop automation or a real CUA transport.
package computeruse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

type readinessClient struct {
	RuntimeClient
	mu       sync.Mutex
	failures int
	version  string
	platform nexuscua.Platform
	protocol string
}

func (client *readinessClient) GetCapabilities(context.Context, time.Duration) (nexuscua.DriverCapabilities, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.failures > 0 {
		client.failures--
		return nexuscua.DriverCapabilities{}, errors.New("not ready")
	}
	return nexuscua.DriverCapabilities{
		ProtocolVersion: client.protocol, RuntimeVersion: client.version, Platform: client.platform,
	}, nil
}

func TestSupervisorStartsVerifiedSidecarAndUsesFreshRestartEpoch(t *testing.T) {
	t.Setenv("NEXUS_CUA_PACKAGE_HELPER", "1")
	t.Setenv("NEXUS_CUA_SIDECAR_HELPER", "1")
	packages := NewPackageManager(PackageConfig{
		Available: true, CommandPath: mustExecutablePath(t), TargetVersion: packageTestVersion,
	})
	factory := func(string, string) (RuntimeClient, error) {
		return &readinessClient{
			failures: 1, version: packageTestVersion,
			platform: nexuscua.Platform(normalizedReleaseOS(runtime.GOOS)), protocol: ProtocolVersion,
		}, nil
	}
	supervisor := NewSupervisor(packages, SupervisorConfig{
		Root: filepath.Join(t.TempDir(), "sidecar"), StartupTimeout: 2 * time.Second,
		ShutdownTimeout: time.Second, ClientFactory: factory,
	})
	client, firstEpoch, capabilities, err := supervisor.EnsureReady(context.Background())
	if err != nil || client == nil {
		t.Fatalf("EnsureReady() error = %v", err)
	}
	if firstEpoch != 1 || capabilities.ProtocolVersion != ProtocolVersion || supervisor.Status().State != SidecarReady {
		t.Fatalf("first status = %+v, capabilities = %+v", supervisor.Status(), capabilities)
	}
	if err = supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	_, secondEpoch, _, err := supervisor.EnsureReady(context.Background())
	if err != nil {
		t.Fatalf("second EnsureReady() error = %v", err)
	}
	if secondEpoch != firstEpoch+1 {
		t.Fatalf("second epoch = %d, want %d", secondEpoch, firstEpoch+1)
	}
	if err = supervisor.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestSupervisorReportsEarlyExitAndRetainsBoundedDiagnosticTail(t *testing.T) {
	t.Setenv("NEXUS_CUA_PACKAGE_HELPER", "1")
	t.Setenv("NEXUS_CUA_SIDECAR_HELPER", "exit")
	packages := NewPackageManager(PackageConfig{Available: true, CommandPath: mustExecutablePath(t)})
	supervisor := NewSupervisor(packages, SupervisorConfig{
		Root: filepath.Join(t.TempDir(), "sidecar"), StartupTimeout: time.Second,
		ClientFactory: func(string, string) (RuntimeClient, error) {
			return &readinessClient{failures: 100}, nil
		},
	})
	if _, _, _, err := supervisor.EnsureReady(context.Background()); err == nil || !strings.Contains(err.Error(), "exited before readiness") {
		t.Fatalf("EnsureReady() error = %v", err)
	}
	status := supervisor.Status()
	if status.State != SidecarFailed || !strings.Contains(status.StderrTail, "helper exited before ready") {
		t.Fatalf("Status() = %+v", status)
	}
}

func TestSupervisorTimesOutAndTerminatesUnreadyProcess(t *testing.T) {
	t.Setenv("NEXUS_CUA_PACKAGE_HELPER", "1")
	t.Setenv("NEXUS_CUA_SIDECAR_HELPER", "1")
	packages := NewPackageManager(PackageConfig{Available: true, CommandPath: mustExecutablePath(t)})
	supervisor := NewSupervisor(packages, SupervisorConfig{
		Root: filepath.Join(t.TempDir(), "sidecar"), StartupTimeout: 120 * time.Millisecond,
		ShutdownTimeout: time.Second,
		ClientFactory: func(string, string) (RuntimeClient, error) {
			return &readinessClient{failures: 100}, nil
		},
	})
	if _, _, _, err := supervisor.EnsureReady(context.Background()); err == nil || !errors.Is(err, ErrSidecarNotReady) {
		t.Fatalf("EnsureReady() error = %v", err)
	}
	if status := supervisor.Status(); status.State != SidecarFailed || status.PID != 0 {
		t.Fatalf("Status() = %+v", status)
	}
}

func TestTailBufferKeepsOnlyBoundedSuffix(t *testing.T) {
	buffer := newTailBuffer(8)
	_, _ = buffer.Write([]byte("12345"))
	_, _ = buffer.Write([]byte("67890"))
	if got := buffer.String(); got != "34567890" {
		t.Fatalf("String() = %q", got)
	}
	_, _ = buffer.Write([]byte("abcdefghijkl"))
	if got := buffer.String(); got != "efghijkl" {
		t.Fatalf("String() = %q", got)
	}
}

func TestSupervisorFreshEpochAfterUnexpectedExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows process-exit ordering is covered by the native x64 gate")
	}
	t.Setenv("NEXUS_CUA_PACKAGE_HELPER", "1")
	t.Setenv("NEXUS_CUA_SIDECAR_HELPER", "1")
	packages := NewPackageManager(PackageConfig{Available: true, CommandPath: mustExecutablePath(t)})
	supervisor := NewSupervisor(packages, SupervisorConfig{
		Root: filepath.Join(t.TempDir(), "sidecar"), StartupTimeout: time.Second,
		ClientFactory: func(string, string) (RuntimeClient, error) {
			return &readinessClient{
				version: packageTestVersion, platform: nexuscua.Platform(normalizedReleaseOS(runtime.GOOS)), protocol: ProtocolVersion,
			}, nil
		},
	})
	_, firstEpoch, _, err := supervisor.EnsureReady(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	process := supervisor.process
	supervisor.mu.Unlock()
	if process == nil {
		t.Fatal("missing supervised process")
	}
	if err = process.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for supervisor.Status().State != SidecarFailed && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	_, secondEpoch, _, err := supervisor.EnsureReady(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if secondEpoch <= firstEpoch {
		t.Fatalf("epoch did not advance: %d -> %d", firstEpoch, secondEpoch)
	}
	_ = supervisor.Close(context.Background())
}
