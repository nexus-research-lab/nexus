package clientopts

import "testing"

func TestScrubInheritedRuntimeEnvRemovesControlAndRelayConfiguration(t *testing.T) {
	keys := []string{
		"CONTROL_SERVICE_TOKEN",
		"CONTROL_SETUP_TOKEN",
		"CONTROL_SIGNING_PRIVATE_KEY",
		"NEXUS_CONTROL_SERVICE_TOKEN",
		"NEXUS_CONTROL_SERVICE_TOKEN_FILE",
		"NEXUS_REMOTE_URL",
		"NEXUS_RELAY_URL",
		"NEXUS_RELAY_REQUEST_TIMEOUT_SECONDS",
		"RELAY_DATABASE_DRIVER",
		"RELAY_DATABASE_URL",
		"RELAY_DEPLOYMENT_ID",
		"RELAY_PRINCIPAL_PUBLIC_KEY",
		"RELAY_PRINCIPAL_PUBLIC_KEY_FILE",
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
