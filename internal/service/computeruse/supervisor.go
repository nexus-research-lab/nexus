// INPUT: a verified nexus-cua binary selection and host-private lifecycle root.
// OUTPUT: bounded sidecar start/readiness/restart/shutdown plus typed client and epoch.
// POS: Nexus-owned process supervision boundary; no Agent identity or CUA policy lives here.
package computeruse

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

const (
	defaultStartupTimeout  = 10 * time.Second
	defaultShutdownTimeout = 5 * time.Second
	readinessProbeTimeout  = 500 * time.Millisecond
	restartWindow          = time.Minute
	maxRestartsPerWindow   = 3
	sidecarLogTailBytes    = 32 << 10
)

type SupervisorConfig struct {
	Root            string
	GOOS            string
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
	Now             func() time.Time
	ClientFactory   runtimeClientFactory
}

type supervisedProcess struct {
	command      *exec.Cmd
	done         chan error
	epoch        uint64
	root         string
	artifactRoot string
	client       RuntimeClient
	capabilities nexuscua.DriverCapabilities
	stdout       *tailBuffer
	stderr       *tailBuffer
}

type Supervisor struct {
	packages *PackageManager
	config   SupervisorConfig

	lifecycleMu sync.Mutex
	mu          sync.Mutex
	status      SidecarStatus
	process     *supervisedProcess
	restarts    []time.Time
}

func NewSupervisor(packages *PackageManager, config SupervisorConfig) *Supervisor {
	config.Root = cleanOptionalPath(config.Root)
	if config.GOOS == "" {
		config.GOOS = runtime.GOOS
	}
	if config.StartupTimeout <= 0 {
		config.StartupTimeout = defaultStartupTimeout
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = defaultShutdownTimeout
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.ClientFactory == nil {
		config.ClientFactory = newSDKRuntimeClient
	}
	return &Supervisor{packages: packages, config: config, status: SidecarStatus{State: SidecarStopped}}
}

func (supervisor *Supervisor) Status() SidecarStatus {
	if supervisor == nil {
		return SidecarStatus{State: SidecarStopped}
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	return supervisor.statusLocked()
}

func (supervisor *Supervisor) Current() (RuntimeClient, uint64, bool) {
	if supervisor == nil {
		return nil, 0, false
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if supervisor.status.State != SidecarReady || supervisor.process == nil {
		return nil, supervisor.status.Epoch, false
	}
	return supervisor.process.client, supervisor.process.epoch, true
}

func (supervisor *Supervisor) statusLocked() SidecarStatus {
	status := supervisor.status
	if supervisor.process != nil {
		status.StdoutTail = supervisor.process.stdout.String()
		status.StderrTail = supervisor.process.stderr.String()
	}
	return status
}

// EnsureReady returns the current typed client or starts a fresh epoch. It
// serializes lifecycle changes while Status remains non-blocking.
func (supervisor *Supervisor) EnsureReady(ctx context.Context) (RuntimeClient, uint64, nexuscua.DriverCapabilities, error) {
	if supervisor == nil || supervisor.packages == nil {
		return nil, 0, nexuscua.DriverCapabilities{}, ErrSidecarNotReady
	}
	supervisor.lifecycleMu.Lock()
	defer supervisor.lifecycleMu.Unlock()

	supervisor.mu.Lock()
	if supervisor.status.State == SidecarReady && supervisor.process != nil {
		process := supervisor.process
		supervisor.mu.Unlock()
		return process.client, process.epoch, process.capabilities, nil
	}
	if !supervisor.restartAllowedLocked() {
		supervisor.mu.Unlock()
		return nil, 0, nexuscua.DriverCapabilities{}, ErrRestartThrottled
	}
	supervisor.mu.Unlock()

	selection, err := supervisor.packages.CurrentBinary(ctx)
	if err != nil {
		return nil, 0, nexuscua.DriverCapabilities{}, err
	}
	process, err := supervisor.startProcess(selection)
	if err != nil {
		supervisor.mu.Lock()
		supervisor.status.State = SidecarFailed
		supervisor.status.LastError = "Computer Use sidecar could not start"
		supervisor.restarts = append(supervisor.restarts, supervisor.config.Now())
		supervisor.mu.Unlock()
		return nil, 0, nexuscua.DriverCapabilities{}, fmt.Errorf("%w: process start failed", ErrSidecarNotReady)
	}
	capabilities, err := supervisor.awaitReadiness(ctx, process, selection)
	if err != nil {
		supervisor.stopProcess(process)
		supervisor.mu.Lock()
		if supervisor.process == process {
			supervisor.status.State = SidecarFailed
			supervisor.status.LastError = PublicErrorMessage(err)
			supervisor.status.StdoutTail = process.stdout.String()
			supervisor.status.StderrTail = process.stderr.String()
			supervisor.process = nil
		}
		supervisor.mu.Unlock()
		_ = os.RemoveAll(process.root)
		return nil, process.epoch, nexuscua.DriverCapabilities{}, err
	}
	process.capabilities = capabilities
	supervisor.mu.Lock()
	if supervisor.process != process || supervisor.status.State == SidecarFailed {
		supervisor.mu.Unlock()
		return nil, process.epoch, nexuscua.DriverCapabilities{}, ErrSidecarNotReady
	}
	supervisor.status.State = SidecarReady
	supervisor.status.Version = capabilities.RuntimeVersion
	supervisor.status.ProtocolVersion = capabilities.ProtocolVersion
	supervisor.status.LastError = ""
	supervisor.mu.Unlock()
	return process.client, process.epoch, capabilities, nil
}

func (supervisor *Supervisor) startProcess(selection BinarySelection) (*supervisedProcess, error) {
	if supervisor.config.Root == "" || supervisor.config.Root == "." {
		return nil, errors.New("Computer Use sidecar root is not configured")
	}
	if err := os.MkdirAll(supervisor.config.Root, 0o700); err != nil {
		return nil, fmt.Errorf("create Computer Use sidecar root: %w", err)
	}
	token, err := randomSecret(32)
	if err != nil {
		return nil, err
	}
	supervisor.mu.Lock()
	epoch := supervisor.status.Epoch + 1
	supervisor.mu.Unlock()
	epochName := fmt.Sprintf("epoch-%d-%s", epoch, token[:16])
	epochRoot := filepath.Join(supervisor.config.Root, "epochs", epochName)
	artifactRoot := filepath.Join(epochRoot, "artifacts")
	if err = os.MkdirAll(artifactRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create Computer Use epoch: %w", err)
	}
	tokenFile := filepath.Join(epochRoot, "token")
	if err = os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		_ = os.RemoveAll(epochRoot)
		return nil, fmt.Errorf("write Computer Use transport token: %w", err)
	}
	endpoint := filepath.Join(epochRoot, "service.sock")
	if normalizedReleaseOS(supervisor.config.GOOS) == "windows" {
		endpoint = `\\.\pipe\nexus-cua-` + token[:24]
	}
	stdout := newTailBuffer(sidecarLogTailBytes)
	stderr := newTailBuffer(sidecarLogTailBytes)
	command := exec.Command(
		selection.Path,
		"serve",
		"--endpoint", endpoint,
		"--token-file", tokenFile,
		"--artifact-root", artifactRoot,
	)
	command.Stdout = stdout
	command.Stderr = stderr
	if err = command.Start(); err != nil {
		_ = os.RemoveAll(epochRoot)
		return nil, fmt.Errorf("start Computer Use sidecar: %w", err)
	}
	client, err := supervisor.config.ClientFactory(endpoint, tokenFile)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = os.RemoveAll(epochRoot)
		return nil, fmt.Errorf("create Computer Use client: %w", err)
	}
	process := &supervisedProcess{
		command: command, done: make(chan error, 1), epoch: epoch, root: epochRoot,
		client: client, stdout: stdout, stderr: stderr, artifactRoot: artifactRoot,
	}
	supervisor.mu.Lock()
	supervisor.process = process
	supervisor.status = SidecarStatus{
		State: SidecarStarting, Epoch: epoch, PID: command.Process.Pid,
		Version: selection.Version, ProtocolVersion: selection.ProtocolVersion,
		StartedAt: supervisor.config.Now().UTC(), UnexpectedExits: supervisor.status.UnexpectedExits,
	}
	supervisor.mu.Unlock()
	go supervisor.waitForExit(process)
	return process, nil
}

func (supervisor *Supervisor) awaitReadiness(
	ctx context.Context,
	process *supervisedProcess,
	selection BinarySelection,
) (nexuscua.DriverCapabilities, error) {
	deadline := supervisor.config.Now().Add(supervisor.config.StartupTimeout)
	var lastError error
	for supervisor.config.Now().Before(deadline) {
		select {
		case exitErr := <-process.done:
			return nexuscua.DriverCapabilities{}, fmt.Errorf("Computer Use sidecar exited before readiness: %w", exitErr)
		case <-ctx.Done():
			return nexuscua.DriverCapabilities{}, ctx.Err()
		default:
		}
		probeCtx, cancel := context.WithTimeout(ctx, readinessProbeTimeout)
		capabilities, err := process.client.GetCapabilities(probeCtx, readinessProbeTimeout)
		cancel()
		if err == nil {
			if capabilities.ProtocolVersion != ProtocolVersion {
				return nexuscua.DriverCapabilities{}, ErrProtocolMismatch
			}
			if strings.TrimSpace(capabilities.RuntimeVersion) != strings.TrimSpace(selection.Version) {
				return nexuscua.DriverCapabilities{}, fmt.Errorf(
					"Computer Use sidecar version %q does not match verified package %q",
					capabilities.RuntimeVersion,
					selection.Version,
				)
			}
			if string(capabilities.Platform) != normalizedReleaseOS(supervisor.config.GOOS) {
				return nexuscua.DriverCapabilities{}, fmt.Errorf("Computer Use sidecar platform mismatch")
			}
			return capabilities, nil
		}
		lastError = err
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nexuscua.DriverCapabilities{}, ctx.Err()
		case exitErr := <-process.done:
			timer.Stop()
			return nexuscua.DriverCapabilities{}, fmt.Errorf("Computer Use sidecar exited before readiness: %w", exitErr)
		case <-timer.C:
		}
	}
	if lastError != nil {
		return nexuscua.DriverCapabilities{}, fmt.Errorf("%w: startup deadline exceeded: %v", ErrSidecarNotReady, lastError)
	}
	return nexuscua.DriverCapabilities{}, fmt.Errorf("%w: startup deadline exceeded", ErrSidecarNotReady)
}

func (supervisor *Supervisor) waitForExit(process *supervisedProcess) {
	err := process.command.Wait()
	process.done <- err
	close(process.done)

	supervisor.mu.Lock()
	if supervisor.process != process {
		supervisor.mu.Unlock()
		return
	}
	stopping := supervisor.status.State == SidecarStopping
	if stopping {
		supervisor.status.State = SidecarStopped
		supervisor.status.LastError = ""
	} else {
		supervisor.status.State = SidecarFailed
		supervisor.status.UnexpectedExits++
		supervisor.status.LastError = exitMessage(err)
		supervisor.restarts = append(supervisor.restarts, supervisor.config.Now())
	}
	supervisor.status.PID = 0
	supervisor.status.StdoutTail = process.stdout.String()
	supervisor.status.StderrTail = process.stderr.String()
	supervisor.process = nil
	supervisor.mu.Unlock()
	_ = os.RemoveAll(process.root)
}

func (supervisor *Supervisor) Stop(ctx context.Context) error {
	if supervisor == nil {
		return nil
	}
	supervisor.lifecycleMu.Lock()
	defer supervisor.lifecycleMu.Unlock()
	supervisor.mu.Lock()
	process := supervisor.process
	if process == nil {
		supervisor.status.State = SidecarStopped
		supervisor.status.PID = 0
		supervisor.mu.Unlock()
		return nil
	}
	supervisor.status.State = SidecarStopping
	supervisor.mu.Unlock()
	return supervisor.stopProcessWithContext(ctx, process)
}

func (supervisor *Supervisor) stopProcess(process *supervisedProcess) {
	ctx, cancel := context.WithTimeout(context.Background(), supervisor.config.ShutdownTimeout)
	defer cancel()
	_ = supervisor.stopProcessWithContext(ctx, process)
}

func (supervisor *Supervisor) stopProcessWithContext(ctx context.Context, process *supervisedProcess) error {
	if process == nil || process.command == nil || process.command.Process == nil {
		return nil
	}
	_ = signalSidecar(process.command.Process)
	timer := time.NewTimer(supervisor.config.ShutdownTimeout)
	defer timer.Stop()
	select {
	case <-process.done:
		return nil
	case <-ctx.Done():
		_ = process.command.Process.Kill()
		return ctx.Err()
	case <-timer.C:
		if err := process.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		select {
		case <-process.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (supervisor *Supervisor) Close(ctx context.Context) error {
	return supervisor.Stop(ctx)
}

func (supervisor *Supervisor) restartAllowedLocked() bool {
	cutoff := supervisor.config.Now().Add(-restartWindow)
	kept := supervisor.restarts[:0]
	for _, instant := range supervisor.restarts {
		if instant.After(cutoff) {
			kept = append(kept, instant)
		}
	}
	supervisor.restarts = kept
	return len(kept) < maxRestartsPerWindow
}

func randomSecret(bytes int) (string, error) {
	payload := make([]byte, bytes)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(payload), nil
}

func exitMessage(err error) string {
	if err == nil {
		return "Computer Use sidecar exited unexpectedly"
	}
	return boundedText("Computer Use sidecar exited unexpectedly: "+err.Error(), 2048)
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (buffer *tailBuffer) Write(payload []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.limit <= 0 {
		return len(payload), nil
	}
	if len(payload) >= buffer.limit {
		buffer.data = append(buffer.data[:0], payload[len(payload)-buffer.limit:]...)
		return len(payload), nil
	}
	overflow := len(buffer.data) + len(payload) - buffer.limit
	if overflow > 0 {
		copy(buffer.data, buffer.data[overflow:])
		buffer.data = buffer.data[:len(buffer.data)-overflow]
	}
	buffer.data = append(buffer.data, payload...)
	return len(payload), nil
}

func (buffer *tailBuffer) String() string {
	if buffer == nil {
		return ""
	}
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return boundedText(string(buffer.data), buffer.limit)
}

var _ io.Writer = (*tailBuffer)(nil)
