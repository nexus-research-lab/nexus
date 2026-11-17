package hostmemory

import (
	"runtime"
	"testing"
)

func TestNativeMemoryProbe(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("no native probe")
	}
	available, err := NewAdmissionProbe(1024)()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("host accepts new work: %v", available)
}

func TestMemoryHeadroomRejectsPressureAndInvalidReadings(t *testing.T) {
	for _, tt := range []struct {
		total, available, reserve uint64
		want, wantErr             bool
	}{
		{8 << 30, 2 << 30, 1024, true, false},
		{16 << 30, 2 << 30, 1024, false, false},
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
