package workspace

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

var defaultDirs = []string{".agents", ".claude"}

func removeDirIfEmpty(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	if err = os.Remove(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func projectRoot() string {
	return appfs.Root()
}
