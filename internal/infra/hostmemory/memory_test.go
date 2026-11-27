package hostmemory

import (
	"runtime"
	"testing"
)

func TestNativeMemoryProbe(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("no native probe")
	}
	if _, err := memoryAvailable(1024); err != nil {
		t.Fatal(err)
	}
	if total, err := totalMemory(); err != nil || total == 0 {
		t.Fatalf("totalMemory = %d, %v", total, err)
	}
	limit := DefaultSessionLimit()
	if limit < minDefaultSessions || limit > maxDefaultSessions {
		t.Fatalf("DefaultSessionLimit = %d", limit)
	}
	t.Logf("host accepts new work: %v, default sessions: %d", NewAdmissionProbe(1024, nil)(), limit)
}
