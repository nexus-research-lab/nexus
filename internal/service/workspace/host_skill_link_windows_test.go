//go:build windows

package workspace

import (
	"os/exec"
	"testing"
)

func createTestHostSkillDirectoryLink(t *testing.T, target string, link string) {
	t.Helper()
	output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("当前 Windows 环境无法创建测试 junction: %v: %s", err, output)
	}
}
