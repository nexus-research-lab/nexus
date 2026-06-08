package goal

import "testing"

func TestShouldIgnoreRuntimeForPermissionMode(t *testing.T) {
	if !ShouldIgnoreRuntimeForPermissionMode("plan") {
		t.Fatal("plan mode should ignore Goal runtime")
	}
	for _, mode := range []string{"", "default", "acceptEdits", "bypassPermissions"} {
		if ShouldIgnoreRuntimeForPermissionMode(mode) {
			t.Fatalf("mode %q should not ignore Goal runtime", mode)
		}
	}
}
