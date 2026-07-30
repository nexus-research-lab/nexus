package skills

import (
	"context"
	"strings"
	"testing"
)

func TestRemoteGitCommitExplainsDeletedBranch(t *testing.T) {
	service := &Service{}
	service.commandRunner = func(
		_ context.Context,
		_ string,
		_ []string,
		_ ...string,
	) (string, error) {
		return "", nil
	}

	_, err := service.remoteGitCommit(context.Background(), externalManifest{
		GitBranch:  "deleted-branch",
		GitURL:     "https://example.com/skills.git",
		SourceKind: externalSourceKindGit,
	})
	if err == nil {
		t.Fatal("远端分支不存在时应返回明确错误")
	}
	for _, expected := range []string{
		"远端分支已不存在",
		"deleted-branch",
		"无法检查更新",
		"重新导入",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("错误信息缺少 %q: %v", expected, err)
		}
	}
}
