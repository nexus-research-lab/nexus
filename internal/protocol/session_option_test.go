package protocol

import "testing"

func TestSessionRuntimeSettingsPreserveUnrelatedOptions(t *testing.T) {
	options := map[string]any{
		OptionRuntimeKind: "nxs",
		"custom":          "value",
	}
	updated := WithSessionRuntimeSettings(options, SessionRuntimeSettings{
		Provider:       "openai",
		Model:          "gpt-5",
		PermissionMode: "acceptEdits",
	})
	settings := SessionRuntimeSettingsFromOptions(updated)
	if settings.Provider != "openai" ||
		settings.Model != "gpt-5" ||
		settings.PermissionMode != "acceptEdits" {
		t.Fatalf("SessionRuntimeSettingsFromOptions() = %+v", settings)
	}
	if updated["custom"] != "value" || updated[OptionRuntimeKind] != "nxs" {
		t.Fatalf("unrelated options changed: %#v", updated)
	}
	if _, exists := options[OptionSessionProvider]; exists {
		t.Fatalf("WithSessionRuntimeSettings() mutated input: %#v", options)
	}

	cleared := WithSessionRuntimeSettings(updated, SessionRuntimeSettings{})
	for _, key := range []string{
		OptionSessionProvider,
		OptionSessionModel,
		OptionSessionPermissionMode,
	} {
		if _, exists := cleared[key]; exists {
			t.Fatalf("cleared options still contain %q: %#v", key, cleared)
		}
	}
}
