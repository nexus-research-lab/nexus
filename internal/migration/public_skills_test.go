package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublicSkillProjectionMigrationRetriesWithoutDataLoss(t *testing.T) {
	state := t.TempDir()
	app := filepath.Join(state, "app")
	ctx := workspaceFileMigrationContext{configRoot: app}
	for _, name := range []string{"platform-skills", "host-skills"} {
		writeMigrationTestFile(t, filepath.Join(app, name, "SKILL.md"), name)
	}
	if n, err := migratePublicSkillProjections(ctx); err != nil || n != 2 {
		t.Fatalf("moved=%d err=%v", n, err)
	}
	if n, err := migratePublicSkillProjections(ctx); err != nil || n != 0 {
		t.Fatalf("retry moved=%d err=%v", n, err)
	}
	for _, name := range []string{"platform-skills", "host-skills"} {
		assertMigrationFileContent(t, filepath.Join(state, "skills-public", name, "SKILL.md"), name)
		assertMigrationPathMissing(t, filepath.Join(app, name))
	}
	writeMigrationTestFile(t, filepath.Join(app, "host-skills", "SKILL.md"), "keep both")
	if _, err := migratePublicSkillProjections(ctx); err == nil {
		t.Fatal("conflicting projection overwritten")
	}
	assertMigrationFileContent(t, filepath.Join(app, "host-skills", "SKILL.md"), "keep both")
}

func TestPublicSkillProjectionMigrationRejectsLinkedDestination(t *testing.T) {
	state, outside := t.TempDir(), t.TempDir()
	app := filepath.Join(state, "app")
	writeMigrationTestFile(t, filepath.Join(app, "platform-skills", "SKILL.md"), "keep")
	if err := os.Symlink(outside, filepath.Join(state, "skills-public")); err != nil {
		t.Skip(err)
	}
	if _, err := migratePublicSkillProjections(workspaceFileMigrationContext{configRoot: app}); err == nil {
		t.Fatal("followed linked destination")
	}
	assertMigrationFileContent(t, filepath.Join(app, "platform-skills", "SKILL.md"), "keep")
}
