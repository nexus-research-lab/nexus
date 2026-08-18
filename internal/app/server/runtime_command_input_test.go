package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestPrepareRuntimeCommandInputIsPrivateStableAndRoundScoped(t *testing.T) {
	t.Setenv(appfs.NexusStateRootEnvName, t.TempDir())
	firstPath, cleanup, err := prepareRuntimeCommandInput("owner-a", "round-a", "capability-a")
	if err != nil {
		t.Fatalf("prepareRuntimeCommandInput() error = %v", err)
	}
	defer cleanup()
	info, err := os.Stat(firstPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("input staging info = %+v err=%v", info, err)
	}
	if content, readErr := os.ReadFile(firstPath); readErr != nil || string(content) != "{}\n" {
		t.Fatalf("initial input staging = %q err=%v", content, readErr)
	}
	custom := []byte(`{"instruction":"date '+%Y-%m-%d'"}`)
	if err = os.WriteFile(firstPath, custom, 0o600); err != nil {
		t.Fatal(err)
	}
	reusedPath, reusedCleanup, err := prepareRuntimeCommandInput("owner-a", "round-a", "capability-a")
	if err != nil {
		t.Fatal(err)
	}
	defer reusedCleanup()
	if reusedPath != firstPath {
		t.Fatalf("same round path changed: first=%s reused=%s", firstPath, reusedPath)
	}
	if content, readErr := os.ReadFile(reusedPath); readErr != nil || string(content) != string(custom) {
		t.Fatalf("same round input was overwritten: %q err=%v", content, readErr)
	}
	secondPath, secondCleanup, err := prepareRuntimeCommandInput("owner-a", "round-b", "capability-a")
	if err != nil {
		t.Fatal(err)
	}
	defer secondCleanup()
	if secondPath == firstPath || filepath.Dir(secondPath) == filepath.Dir(firstPath) {
		t.Fatalf("different rounds reused staging directory: %s", firstPath)
	}
	staleDirectory := filepath.Join(filepath.Dir(filepath.Dir(firstPath)), "stale-round")
	if err = os.Mkdir(staleDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	staleAt := time.Now().Add(-runtimeCommandInputRetention - time.Hour)
	if err = os.Chtimes(staleDirectory, staleAt, staleAt); err != nil {
		t.Fatal(err)
	}
	_, thirdCleanup, err := prepareRuntimeCommandInput("owner-a", "round-c", "capability-a")
	if err != nil {
		t.Fatal(err)
	}
	defer thirdCleanup()
	if _, err = os.Stat(staleDirectory); !os.IsNotExist(err) {
		t.Fatalf("stale round staging was not reaped: %v", err)
	}
	cleanup()
	if _, err = os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("round cleanup left input staging: %v", err)
	}
}
