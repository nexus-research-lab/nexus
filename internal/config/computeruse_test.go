package config

import "testing"

func TestComputerUseIsOptIn(t *testing.T) {
	t.Setenv("NEXUS_COMPUTER_USE_ENABLED", "")
	t.Setenv("NEXUS_COMPUTER_USE_BINARY", "")
	cfg := Load()
	if cfg.ComputerUseEnabled {
		t.Fatal("Computer Use must be disabled when the flag is empty")
	}
	if cfg.ComputerUseBinaryPath != "" {
		t.Fatalf("ComputerUseBinaryPath = %q, want empty", cfg.ComputerUseBinaryPath)
	}
}

func TestComputerUseConfiguration(t *testing.T) {
	t.Setenv("NEXUS_COMPUTER_USE_ENABLED", "true")
	t.Setenv("NEXUS_COMPUTER_USE_BINARY", " /Applications/Nexus CUA.app/Contents/MacOS/nexuscua ")
	cfg := Load()
	if !cfg.ComputerUseEnabled {
		t.Fatal("ComputerUseEnabled = false, want true")
	}
	if cfg.ComputerUseBinaryPath != "/Applications/Nexus CUA.app/Contents/MacOS/nexuscua" {
		t.Fatalf("ComputerUseBinaryPath = %q", cfg.ComputerUseBinaryPath)
	}
}
