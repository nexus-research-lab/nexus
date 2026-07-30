package workspace

import (
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

var defaultDirs = []string{".agents", ".claude"}

func projectRoot() string {
	return appfs.Root()
}
