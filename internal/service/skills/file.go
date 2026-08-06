// INPUT: Skill 名称、源目录与已经创建的私有暂存目录。
// OUTPUT: 经校验的 SKILL.md 内容与无链接目录副本。
// POS: 外部 Skill 进入 owner registry 前的文件校验层。
package skills

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func validateSkillName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("skill name 不能为空")
	}
	if trimmed != name {
		return errors.New("skill name 不能包含首尾空白")
	}
	if trimmed == "." || trimmed == ".." || strings.ContainsAny(trimmed, "/\\\x00") {
		return errors.New("skill name 不能包含路径分隔符")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "external:") {
		return errors.New("skill name 不能使用保留引用前缀")
	}
	return nil
}

func readSkillSource(sourceDir string) (string, string, string, error) {
	skillMDPath := filepath.Join(sourceDir, "SKILL.md")
	root, err := confinedfs.Open(sourceDir)
	if err != nil {
		return "", "", "", err
	}
	defer root.Close()
	content, err := readConfinedRegularFile(root, "SKILL.md")
	if err != nil {
		return "", "", "", err
	}
	return string(content), skillMDPath, filepath.Base(sourceDir), nil
}

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

// copyDirectoryAt 将外部源目录复制到已固定的 owner 根。
func copyDirectoryAt(
	sourceDir string,
	targetRoot *confinedfs.Root,
	targetRelative string,
) error {
	sourceRoot, err := confinedfs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	target, err := targetRoot.OpenOrCreateRootNoSymlink(
		targetRelative,
		appfs.RuntimeCollaborativeDirectoryMode(0o700),
	)
	if err != nil {
		return err
	}
	defer target.Close()
	return target.CopyTreeFrom(sourceRoot)
}

// writeSkillDirectoryFileAt 在已固定的 Skill 根中原子写入文件。
func writeSkillDirectoryFileAt(
	root *confinedfs.Root,
	skillRelative string,
	fileName string,
	payload []byte,
	mode os.FileMode,
) error {
	skillRoot, err := root.OpenRootNoSymlink(skillRelative)
	if err != nil {
		return err
	}
	defer skillRoot.Close()
	return skillRoot.WriteFileAtomic(fileName, payload, mode)
}

func matchSkillQuery(detail Detail, query string) bool {
	fields := []string{
		strings.ToLower(detail.Name),
		strings.ToLower(detail.Title),
		strings.ToLower(detail.Description),
		strings.ToLower(strings.Join(detail.Tags, " ")),
	}
	return slices.ContainsFunc(fields, func(field string) bool {
		return strings.Contains(field, query)
	})
}

func defaultSkillScope(scope string) string {
	normalized := strings.TrimSpace(scope)
	if normalized == scopeMain {
		return scopeMain
	}
	if normalized == scopeRoom {
		return scopeRoom
	}
	return scopeAny
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptySlice(candidates ...[]string) []string {
	for _, item := range candidates {
		if len(item) > 0 {
			return slices.Clone(item)
		}
	}
	return []string{}
}

func projectRoot() string {
	return appfs.Root()
}
