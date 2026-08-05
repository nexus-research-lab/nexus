//go:build !windows

package workspace

import (
	"os"
	"testing"
)

func createTestHostSkillDirectoryLink(t *testing.T, target string, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("当前平台无法创建测试目录链接: %v", err)
	}
}
