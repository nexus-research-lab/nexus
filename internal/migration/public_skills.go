// INPUT: Canonical app root and its legacy managed Skill projections.
// OUTPUT: Versioned relocation to state/skills-public without overwriting either copy.
// POS: Public runtime inputs are separated from the private app boundary before startup.
package migration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func migratePublicSkillProjections(ctx workspaceFileMigrationContext) (int, error) {
	state := filepath.Dir(ctx.configRoot)
	root, err := confinedfs.Open(state)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer root.Close()
	moved := 0
	for _, name := range []string{"platform-skills", "host-skills"} {
		source := filepath.ToSlash(filepath.Join(filepath.Base(ctx.configRoot), name))
		if _, err := root.Lstat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return moved, err
		}
		sourceRoot, err := root.OpenRootNoSymlink(source)
		if err != nil {
			return moved, err
		}
		if err := sourceRoot.Close(); err != nil {
			return moved, err
		}
		public, err := root.OpenOrCreateRootNoSymlink("skills-public", 0755)
		if err != nil {
			return moved, err
		}
		if err := public.Close(); err != nil {
			return moved, err
		}
		target := "skills-public/" + name
		if _, err := root.Lstat(target); err == nil {
			return moved, fmt.Errorf("both legacy and public Skill projections exist: %s", name)
		} else if !os.IsNotExist(err) {
			return moved, err
		}
		if err := root.Rename(source, target); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}
