//go:build windows

// INPUT: desktop script command, exact workspace/environment, cancellation context and bounded output sinks.
// OUTPUT: shell result plus proof failure when the Windows Job Object cannot be assigned, terminated or drained.
// POS: Windows desktop process-tree boundary; every spawned command is fenced by a kill-on-close Job Object.
package automation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsJobDrainTimeout = 2 * time.Second

var errScriptTreeTerminationUnconfirmed = errors.New("script process tree termination could not be confirmed")

type windowsJobBasicAccounting struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func runDesktopScript(
	ctx context.Context,
	script string,
	dir string,
	environment []string,
	stdout io.Writer,
	stderr io.Writer,
) (error, error) {
	if err := ctx.Err(); err != nil {
		return err, nil
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create script job object: %w", err), nil
	}
	defer windows.CloseHandle(job)

	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)),
		uint32(unsafe.Sizeof(limits)),
	); err != nil {
		return fmt.Errorf("configure script job object: %w", err), nil
	}

	command := exec.Command("cmd.exe", "/C", script)
	command.Dir = dir
	command.Env = environment
	command.Stdout = stdout
	command.Stderr = stderr
	command.WaitDelay = 100 * time.Millisecond
	if err = command.Start(); err != nil {
		return err, nil
	}

	processHandle, openErr := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE,
		false,
		uint32(command.Process.Pid),
	)
	if openErr != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return openErr, fmt.Errorf("%w: open script process: %v", errScriptTreeTerminationUnconfirmed, openErr)
	}
	defer windows.CloseHandle(processHandle)
	if assignErr := windows.AssignProcessToJobObject(job, processHandle); assignErr != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return assignErr, fmt.Errorf("%w: assign script process to job: %v", errScriptTreeTerminationUnconfirmed, assignErr)
	}

	processExited := make(chan error, 1)
	go func() {
		status, waitErr := windows.WaitForSingleObject(processHandle, windows.INFINITE)
		if waitErr == nil && status != windows.WAIT_OBJECT_0 {
			waitErr = fmt.Errorf("unexpected process wait status %d", status)
		}
		processExited <- waitErr
	}()

	var waitProofErr error
	select {
	case waitProofErr = <-processExited:
	case <-ctx.Done():
	}
	terminateErr := windows.TerminateJobObject(job, 1)
	runErr := command.Wait()
	if terminateErr != nil {
		return runErr, fmt.Errorf("%w: terminate script job: %v", errScriptTreeTerminationUnconfirmed, terminateErr)
	}
	if waitProofErr != nil {
		return runErr, fmt.Errorf("%w: wait for script shell: %v", errScriptTreeTerminationUnconfirmed, waitProofErr)
	}
	if drainErr := waitWindowsJobStopped(job, windowsJobDrainTimeout); drainErr != nil {
		return runErr, fmt.Errorf("%w: %v", errScriptTreeTerminationUnconfirmed, drainErr)
	}
	if ctx.Err() != nil {
		// os/exec starts cmd.exe before its process handle can be assigned to the
		// Job Object. The job proves all post-assignment descendants are gone,
		// but cannot prove that no descendant escaped in that narrow interval.
		// Keep deletion in review_required instead of claiming a stronger fact.
		return runErr, fmt.Errorf("%w: Windows pre-assignment descendants cannot be excluded", errScriptTreeTerminationUnconfirmed)
	}
	if errors.Is(runErr, exec.ErrWaitDelay) && ctx.Err() == nil {
		runErr = nil
	}
	return runErr, nil
}

func waitWindowsJobStopped(job windows.Handle, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		accounting := windowsJobBasicAccounting{}
		if err := windows.QueryInformationJobObject(
			job,
			windows.JobObjectBasicAccountingInformation,
			uintptr(unsafe.Pointer(&accounting)),
			uint32(unsafe.Sizeof(accounting)),
			nil,
		); err != nil {
			return err
		}
		if accounting.ActiveProcesses == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("job remained active after termination")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
