//go:build windows

package runtime

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsSandboxProcessIdentityMatchesCurrentProcess(t *testing.T) {
	start, err := currentProcessStartTimeUnixNano()
	if err != nil {
		t.Fatal(err)
	}
	if start <= 0 {
		t.Fatalf("current process start time = %d", start)
	}
	actual, alive, known := sandboxProcessIdentity(os.Getpid())
	if !alive || !known || actual != start {
		t.Fatalf("current process identity = (%d, %v, %v), want (%d, true, true)", actual, alive, known, start)
	}
	if sandboxProcessMarkerIsActive(SandboxLeaseMarker{ProcessID: os.Getpid(), ProcessStartTimeUnixNano: start + 1}) {
		t.Fatal("reused PID with mismatched creation time was treated as active")
	}
}

// TestWindowsSandboxExitedProcessHelper 通过 stdin EOF 固定退出时机；259 与 STILL_ACTIVE 同值。
func TestWindowsSandboxExitedProcessHelper(t *testing.T) {
	if os.Getenv("NEXUS_WINDOWS_EXITED_PROCESS_HELPER") != "1" {
		t.Skip("requires the retained-process-handle fixture")
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	os.Exit(259)
}

// TestWindowsSandboxProcessIdentityRejectsRetainedExitedProcess 保留句柄，防止 PID 立即消失掩盖错误。
func TestWindowsSandboxProcessIdentityRejectsRetainedExitedProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsSandboxExitedProcessHelper$")
	cmd.Env = append(os.Environ(), "NEXUS_WINDOWS_EXITED_PROCESS_HELPER=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		input.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	held, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(held)
	start, alive, known := sandboxProcessIdentity(cmd.Process.Pid)
	if start <= 0 || !alive || !known {
		t.Fatalf("live helper identity = %d, %t, %t", start, alive, known)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	exitErr, ok := cmd.Wait().(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 259 {
		t.Fatalf("helper did not exit 259: %v", exitErr)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(held, &code); err != nil || code != 259 {
		t.Fatalf("retained process exit code = %d, %v", code, err)
	}
	_, alive, known = sandboxProcessIdentity(cmd.Process.Pid)
	if alive || !known {
		t.Fatalf("terminated process with a retained handle = alive %t, known %t", alive, known)
	}
	if sandboxProcessMarkerIsActive(SandboxLeaseMarker{ProcessID: cmd.Process.Pid, ProcessStartTimeUnixNano: start}) {
		t.Fatal("terminated generation kept its marker active")
	}
}
