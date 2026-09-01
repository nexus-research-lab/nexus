package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveHostKeysKeepsOneActiveAndAllDistinctLegacySources(t *testing.T) {
	stateRoot := t.TempDir()
	keyPath := filepath.Join(stateRoot, "app", "config", "connector-credentials.key")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(testLegacyKey), 0o600); err != nil {
		t.Fatal(err)
	}
	keychainKey := "enl4d3Z1dHNycXBvbm1sa2ppaGdmZWRjYmEwMTIzNDU="
	resolved, err := resolveHostKeys(
		hostKeyModeAuto,
		stateRoot,
		testActiveKey,
		[]string{testLegacyKey},
		func() (string, error) { return keychainKey, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Active != keychainKey || resolved.Source != "keychain" || len(resolved.Legacy) != 2 {
		t.Fatalf("resolved host keys mismatch: source=%q legacy=%d", resolved.Source, len(resolved.Legacy))
	}
}

func TestResolveHostKeysAutoFallsBackWithoutGuessingInExplicitMode(t *testing.T) {
	resolved, err := resolveHostKeys(
		hostKeyModeAuto,
		t.TempDir(),
		testActiveKey,
		nil,
		func() (string, error) { return "", errors.New("unavailable") },
	)
	if err != nil || resolved.Active != testActiveKey || resolved.Source != "explicit" {
		t.Fatalf("auto explicit fallback mismatch: result=%+v err=%v", resolved, err)
	}
	explicit, err := resolveHostKeys(
		hostKeyModeExplicit,
		t.TempDir(),
		testActiveKey,
		nil,
		func() (string, error) { t.Fatal("explicit mode must not inspect Keychain"); return "", nil },
	)
	if err != nil || explicit.Source != "explicit" {
		t.Fatalf("explicit result mismatch: result=%+v err=%v", explicit, err)
	}
}

func TestResolveHostKeysFileModeDoesNotInspectKeychain(t *testing.T) {
	stateRoot := t.TempDir()
	keyPath := filepath.Join(stateRoot, "app", "config", "connector-credentials.key")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(testActiveKey), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolveHostKeys(
		hostKeyModeFile,
		stateRoot,
		"",
		nil,
		func() (string, error) { t.Fatal("file mode must not inspect Keychain"); return "", nil },
	)
	if err != nil || resolved.Source != "state_file" || resolved.Active != testActiveKey {
		t.Fatalf("file mode mismatch: result=%+v err=%v", resolved, err)
	}
}
