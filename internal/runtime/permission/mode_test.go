package permission

import (
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		name string
		in   sdkpermission.Mode
		want sdkpermission.Mode
	}{
		{name: "empty", in: "", want: sdkpermission.ModeDefault},
		{name: "trimmed", in: " dontAsk ", want: sdkpermission.ModeDontAsk},
		{name: "unknown", in: "unsafe-mode", want: sdkpermission.ModeDefault},
		{name: "auto", in: sdkpermission.ModeAuto, want: sdkpermission.ModeAuto},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeMode(test.in); got != test.want {
				t.Fatalf("NormalizeMode(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
