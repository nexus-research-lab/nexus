//go:build !windows

// INPUT: desktop script command, exact workspace/environment, cancellation context and bounded output sinks.
// OUTPUT: shell result plus proof failure when the isolated Unix process group cannot be fully stopped.
// POS: desktop-only process-tree boundary; server execution continues through workspaceisolation.
package automation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

const unixProcessGroupDrainTimeout = 2 * time.Second

var errScriptTreeTerminationUnconfirmed = errors.New("script process tree termination could not be confirmed")

func runDesktopScript(
	ctx context.Context,
	script string,
	dir string,
	environment []string,
	stdout io.Writer,
	stderr io.Writer,
) (error, error) {
	command := exec.CommandContext(ctx, "/bin/sh", "-c", script)
	command.Dir = dir
	command.Env = environment
	command.Stdout = stdout
	command.Stderr = stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = 100 * time.Millisecond
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		return killUnixProcessGroup(command.Process.Pid)
	}

	runErr := command.Run()
	if command.Process == nil {
		return runErr, nil
	}
	if err := killUnixProcessGroup(command.Process.Pid); err != nil {
		return runErr, fmt.Errorf("%w: %v", errScriptTreeTerminationUnconfirmed, err)
	}
	if err := waitUnixProcessGroupStopped(command.Process.Pid, unixProcessGroupDrainTimeout); err != nil {
		return runErr, fmt.Errorf("%w: %v", errScriptTreeTerminationUnconfirmed, err)
	}
	if errors.Is(runErr, exec.ErrWaitDelay) && ctx.Err() == nil && command.ProcessState != nil && command.ProcessState.Success() {
		// The shell exited successfully but a background descendant retained an
		// output pipe. The group fence above stopped that descendant and proved
		// the group empty, so ErrWaitDelay is not a script failure.
		runErr = nil
	}
	return runErr, nil
}

func killUnixProcessGroup(processGroupID int) error {
	if processGroupID <= 0 {
		return errors.New("invalid process group identity")
	}
	err := syscall.Kill(-processGroupID, syscall.SIGKILL)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func waitUnixProcessGroupStopped(processGroupID int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(-processGroupID, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("process group remained active after termination")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
