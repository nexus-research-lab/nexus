package protocol

import "testing"

func TestParseIMPermissionSlashNameSupportsShortCommandsAndLegacyAliases(t *testing.T) {
	tests := map[string]IMPermissionCommand{
		"/y":       IMPermissionCommandAllowOnce,
		"/Y":       IMPermissionCommandAllowOnce,
		"／y":       IMPermissionCommandAllowOnce,
		"/approve": IMPermissionCommandAllowOnce,
		"/a":       IMPermissionCommandAllowAlways,
		"/always":  IMPermissionCommandAllowAlways,
		"/d":       IMPermissionCommandDeny,
		"/deny":    IMPermissionCommandDeny,
		"/retry":   IMPermissionCommandRetry,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, ok := ParseIMPermissionSlashName(input)
			if !ok || got != want {
				t.Fatalf("ParseIMPermissionSlashName(%q) = %q, %t; want %q, true", input, got, ok, want)
			}
		})
	}
	for _, input := range []string{"", "y", "/unknown"} {
		if got, ok := ParseIMPermissionSlashName(input); ok || got != "" {
			t.Fatalf("ParseIMPermissionSlashName(%q) = %q, %t; want unrecognized", input, got, ok)
		}
	}
}
