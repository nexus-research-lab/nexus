//go:build windows

package runtime

import (
	"os"
	"testing"
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
