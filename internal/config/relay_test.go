package config

import "testing"

func TestLoadRelayConfigurationIsOptional(t *testing.T) {
	t.Setenv("NEXUS_RELAY_URL", "")
	t.Setenv("NEXUS_RELAY_REQUEST_TIMEOUT_SECONDS", "")

	cfg := Load()
	if cfg.RelayURL != "" {
		t.Fatalf("RelayURL = %q, want disabled", cfg.RelayURL)
	}
	if cfg.RelayRequestTimeoutSeconds != 5 {
		t.Fatalf("RelayRequestTimeoutSeconds = %d, want 5", cfg.RelayRequestTimeoutSeconds)
	}
}

func TestLoadRelayConfiguration(t *testing.T) {
	t.Setenv("NEXUS_RELAY_URL", " https://relay.example.com ")
	t.Setenv("NEXUS_RELAY_REQUEST_TIMEOUT_SECONDS", "9")

	cfg := Load()
	if cfg.RelayURL != "https://relay.example.com" {
		t.Fatalf("RelayURL = %q", cfg.RelayURL)
	}
	if cfg.RelayRequestTimeoutSeconds != 9 {
		t.Fatalf("RelayRequestTimeoutSeconds = %d, want 9", cfg.RelayRequestTimeoutSeconds)
	}
}
