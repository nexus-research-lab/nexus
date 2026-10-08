package runtime

import (
	"testing"
)

func TestSandboxProcessMarkerUsesConservativeIdentityFallback(t *testing.T) {
	if !sandboxProcessMarkerIsActive(SandboxLeaseMarker{ProcessID: -1}) {
		t.Fatal("invalid process identity was not retained")
	}
	if !sandboxProcessMarkerIsActive(SandboxLeaseMarker{ProcessID: 0, ProcessStartTimeUnixNano: 1}) {
		t.Fatal("unverifiable process identity was not retained")
	}
}
