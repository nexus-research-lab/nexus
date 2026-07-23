package appfs

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestUserSkillLibraryRootKeepsUnsafeOwnerInsideConfig(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", t.TempDir())
	path := UserSkillLibraryRoot("../owner/a")
	if path == "" || path == UserSkillLibraryRoot("owner_a") {
		t.Fatalf("不安全 owner 应生成隔离目录: %q", path)
	}
	if segment := safePathSegment(".."); segment == "." || segment == ".." {
		t.Fatalf("路径段不能保持目录穿越值: %q", segment)
	}
	for _, ownerID := range []string{"CON", "CON.txt"} {
		if segment := safePathSegment(ownerID); segment == ownerID {
			t.Fatalf("Windows 保留设备名不能直接作为路径段: %q", segment)
		}
	}
}

func TestUserSkillLibraryRootUsesUsersDirectory(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	want := filepath.Join(configDir, "users", "owner-a")
	if got := UserSkillLibraryRoot("owner-a"); got != want {
		t.Fatalf("用户 Skill 根 = %q, want %q", got, want)
	}
}

func TestSkillLibraryRootsKeepsPlatformBeforeUserSource(t *testing.T) {
	t.Setenv("NEXUS_CONFIG_DIR", t.TempDir())
	want := []string{PlatformSkillRoot(), UserSkillLibraryRoot("owner-a")}
	if got := SkillLibraryRoots("owner-a"); !slices.Equal(got, want) {
		t.Fatalf("runtime Skill 根 = %#v, want %#v", got, want)
	}
}
