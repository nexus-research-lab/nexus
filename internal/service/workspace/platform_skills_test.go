package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompatibleSkillLibrarySkipsMatchingPublicationAndRepairsDrift(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "skills")
	targetRoot := filepath.Join(t.TempDir(), "platform-skills")
	writeSkillSourceFile(t, sourceRoot, "demo/SKILL.md", "# demo\n", 0o600)
	writeSkillSourceFile(t, sourceRoot, "demo/scripts/run.sh", "#!/bin/sh\n", 0o700)
	if err := ensureCompatibleSkillLibrary(sourceRoot, targetRoot); err != nil {
		t.Fatalf("首次发布失败: %v", err)
	}

	// 发布会整体替换根目录；哨兵存活即证明内容一致时没有重发。
	sentinel := filepath.Join(targetRoot, "sentinel")
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureCompatibleSkillLibrary(sourceRoot, targetRoot); err != nil {
		t.Fatalf("重复发布失败: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("内容一致时不应重发: %v", err)
	}

	published := filepath.Join(targetRoot, ".agents", "skills", "demo", "SKILL.md")
	if err := os.Chmod(published, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(published, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureCompatibleSkillLibrary(sourceRoot, targetRoot); err != nil {
		t.Fatalf("修复发布失败: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("内容漂移后应整库重发: %v", err)
	}
	content, err := os.ReadFile(published)
	if err != nil || string(content) != "# demo\n" {
		t.Fatalf("重发后内容 = %q, err=%v", content, err)
	}
}

func writeSkillSourceFile(t *testing.T, root string, relative string, content string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
