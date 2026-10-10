// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package skills

import (
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func copyDirectory(sourceDir string, targetDir string) error {
	sourceRoot, err := confinedfs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	if err = os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	targetRoot, err := confinedfs.Open(targetDir)
	if err != nil {
		return err
	}
	defer targetRoot.Close()
	return targetRoot.CopyTreeFrom(sourceRoot)
}
