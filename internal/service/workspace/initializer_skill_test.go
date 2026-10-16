// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// DeploySkill 把指定 skill 部署到目标 workspace。
func DeploySkill(skillName string, sourceDir string, workspacePath string, context map[string]string) error {
	if err := os.MkdirAll(workspacePath, workspaceDirectoryMode()); err != nil {
		return err
	}
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return err
	}
	defer root.Close()
	return DeploySkillAt(root, skillName, sourceDir, context)
}

// DeploySkillAt 把指定 skill 部署到已验证的 workspace 根。
func DeploySkillAt(
	root *confinedfs.Root,
	skillName string,
	sourceDir string,
	context map[string]string,
) error {
	sourceRoot, err := confinedfs.Open(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	return DeploySkillAtFromRoots(root, sourceRoot, skillName, context)
}

// DeploySkillAtFromRoots 在已固定的源与目标目录句柄之间部署 Skill。
//
// 外部 Skill 源可能位于另一个 owner 的共享目录；调用方应先通过
// owner-aware store 固定 sourceRoot，再把它传入，避免校验后重新打开
// record.SourcePath 形成 TOCTOU 或跨 owner 路径旁路。
func DeploySkillAtFromRoots(
	root *confinedfs.Root,
	sourceRoot *confinedfs.Root,
	skillName string,
	context map[string]string,
) error {
	if root == nil || sourceRoot == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	if err := validateWorkspaceSkillName(skillName); err != nil {
		return err
	}
	agentsSkillDir := filepath.ToSlash(filepath.Join(".agents", "skills", skillName))
	claudeSkillEntry := filepath.ToSlash(filepath.Join(".claude", "skills", skillName))
	if err := syncDirectoryRootAt(sourceRoot, root, agentsSkillDir, context); err != nil {
		return err
	}
	return ensureClaudeSkillEntryRootAt(
		sourceRoot,
		root,
		claudeSkillEntry,
		filepath.Join("..", "..", ".agents", "skills", skillName),
		context,
	)
}

// RuntimeSkillNames 合并 Agent 引用与 workspace-local Skill，形成运行时白名单。
//
// 外部引用以 external:<name> 形式持久化，进入 SDK 前还原为 canonical name；
// workspace-local Skill 仍从 workspace 文件发现，避免显式白名单把它过滤掉。
func RuntimeSkillNames(workspacePath string, selectedSkillIDs []string) ([]string, error) {
	root, err := confinedfs.Open(workspacePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return RuntimeSkillNamesAt(root, selectedSkillIDs, nil)
}

func syncDirectoryRootAt(
	sourceRoot *confinedfs.Root,
	root *confinedfs.Root,
	relativeTarget string,
	context map[string]string,
) error {
	if sourceRoot == nil || root == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	var err error
	if err = root.RemoveAll(relativeTarget); err != nil && !os.IsNotExist(err) {
		return err
	}
	targetRoot, err := root.OpenOrCreateRootNoSymlink(relativeTarget, workspaceDirectoryMode())
	if err != nil {
		return err
	}
	defer targetRoot.Close()
	return syncSkillDirectory(sourceRoot, targetRoot, context)
}

func syncSkillDirectory(
	sourceRoot *confinedfs.Root,
	targetRoot *confinedfs.Root,
	context map[string]string,
) error {
	entries, err := fs.ReadDir(sourceRoot.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		info, err := sourceRoot.Lstat(entry.Name())
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			sourceChild, err := sourceRoot.OpenRootNoSymlink(entry.Name())
			if err != nil {
				return err
			}
			targetChild, targetErr := targetRoot.OpenOrCreateRootNoSymlink(
				entry.Name(),
				workspaceDirectoryMode(),
			)
			if targetErr != nil {
				sourceChild.Close()
				return targetErr
			}
			copyErr := syncSkillDirectory(sourceChild, targetChild, context)
			sourceChild.Close()
			targetChild.Close()
			if copyErr != nil {
				return copyErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		sourceFile, err := sourceRoot.OpenFileNoSymlink(entry.Name(), os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		openedInfo, err := sourceFile.Stat()
		if err != nil {
			sourceFile.Close()
			return err
		}
		if entry.Name() == "SKILL.md" {
			content, readErr := io.ReadAll(sourceFile)
			sourceFile.Close()
			if readErr != nil {
				return readErr
			}
			rendered := renderTemplate(string(content), context)
			if err = targetRoot.WriteFileAtomic(
				entry.Name(),
				[]byte(strings.TrimSpace(rendered)+"\n"),
				workspaceFileMode(),
			); err != nil {
				return err
			}
			continue
		}
		err = targetRoot.WriteFileAtomicFrom(
			entry.Name(),
			sourceFile,
			workspaceCopyFileMode(openedInfo.Mode()),
		)
		sourceFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureClaudeSkillEntryRootAt(
	sourceRoot *confinedfs.Root,
	root *confinedfs.Root,
	entryPath string,
	relativeTarget string,
	context map[string]string,
) error {
	if sourceRoot == nil || root == nil {
		return errors.New("skill source 或 workspace 根句柄不能为空")
	}
	err := ensureRelativeSymlinkAt(root, entryPath, relativeTarget)
	if err == nil {
		return nil
	}
	// Windows 默认可能没有目录 symlink 权限，失败时镜像一份给 Claude 读取。
	if mirrorErr := syncDirectoryRootAt(sourceRoot, root, entryPath, context); mirrorErr != nil {
		return fmt.Errorf("创建 Claude Skill 入口失败: %w；镜像目录也失败: %v", err, mirrorErr)
	}
	return nil
}
