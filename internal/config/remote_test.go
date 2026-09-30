package config

import "testing"

func TestDesktopDefaultsToPublicRemoteGateway(t *testing.T) {
	t.Setenv("NEXUS_APP_MODE", "desktop")
	t.Setenv("NEXUS_REMOTE_URL", "")

	if remoteURL := Load().RemoteURL; remoteURL != "https://app.nexusos.cn" {
		t.Fatalf("RemoteURL = %q", remoteURL)
	}
}

func TestDesktopSandboxIsBuiltIn(t *testing.T) {
	t.Setenv("NEXUS_APP_MODE", "desktop")
	t.Setenv("NEXUS_DESKTOP_SANDBOX_ENABLED", "false")
	if !Load().DesktopSandboxEnabled {
		t.Fatal("desktop sandbox must remain enabled without an environment opt-in")
	}
}
