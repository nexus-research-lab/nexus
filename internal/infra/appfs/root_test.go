package appfs

import (
	"path/filepath"
	"testing"
)

func TestStateRootNormalizesLegacyNexusConfigSubdirectory(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, filepath.Join(stateRoot, "config"))

	if got := StateRoot(); got != stateRoot {
		t.Fatalf("StateRoot() 未兼容旧 config 子目录: got=%q want=%q", got, stateRoot)
	}
}
