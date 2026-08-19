package sessionresume

import "testing"

func TestRequiresToolSurfaceFork(t *testing.T) {
	for _, test := range []struct {
		name    string
		stored  string
		current string
		legacy  bool
		want    bool
	}{
		{name: "legacy selected connector session", current: "new", legacy: true, want: true},
		{name: "legacy session baseline", current: "new"},
		{name: "changed surface", stored: "old", current: "new", want: true},
		{name: "stable surface", stored: "same", current: "same"},
		{name: "unknown current surface", stored: "old"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := RequiresToolSurfaceFork(test.stored, test.current, test.legacy); got != test.want {
				t.Fatalf("RequiresToolSurfaceFork() = %v, want %v", got, test.want)
			}
		})
	}
}
