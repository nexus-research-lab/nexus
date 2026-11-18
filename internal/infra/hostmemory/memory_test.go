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

func TestMemoryHeadroomRejectsPressureAndInvalidReadings(t *testing.T) {
	for _, tt := range []struct {
		total, available, reserve uint64
		want, wantErr             bool
	}{
		{8 << 30, 2 << 30, 1024, true, false},
		{16 << 30, 2 << 30, 1024, true, false},
		{16 << 30, 1536 << 20, 1024, false, false},
		// 64 GiB 的比例保留封顶 2 GiB，不再要求近 10 GiB 可用。
		{64 << 30, 3 << 30, 1024, true, false},
		{64 << 30, 1536 << 20, 1024, false, false},
		{4 << 30, 768 << 20, 1024, false, false},
		{8 << 30, 0, 1024, false, false},
		{0, 0, 1024, false, true},
		{8 << 30, 9 << 30, 1024, false, true},
	} {
		got, err := enoughMemory(tt.total, tt.available, tt.reserve)
		if got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("snapshot=%+v got=%v err=%v", tt, got, err)
		}
	}
}

func TestAdmissionProbeDisabledForNonPositiveReserve(t *testing.T) {
	if NewAdmissionProbe(0, nil) != nil {
		t.Fatal("non-positive reserve must disable the probe")
	}
}
