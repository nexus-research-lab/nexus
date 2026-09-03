package clientopts

import "testing"

func TestScrubInheritedRuntimeEnvRemovesControlCredentials(t *testing.T) {
	keys := []string{
		"CONTROL_SERVICE_TOKEN",
		"CONTROL_SETUP_TOKEN",
		"CONTROL_SIGNING_PRIVATE_KEY",
		"NEXUS_CONTROL_SERVICE_TOKEN",
		"NEXUS_CONTROL_SERVICE_TOKEN_FILE",
	}
	for _, key := range keys {
		t.Setenv(key, "must-not-reach-runtime")
	}
	values := scrubInheritedRuntimeEnv()
	for _, key := range keys {
		if value, ok := values[key]; !ok || value != "" {
			t.Fatalf("%s = %q, present = %t", key, value, ok)
		}
	}
}
