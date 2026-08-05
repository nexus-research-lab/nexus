// INPUT: 已校验的宿主 Skill staging 与现有受管投影。
// OUTPUT: canonical 视图、Claude fallback 与单项 last-known-good 状态。
// POS: 宿主 Skill 快照的分 Skill 发布和兼容视图层。
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func reconcileHostSkillEntries(
	root *confinedfs.Root,
	stagingParent string,
	targetParent string,
	view string,
	kept map[string]struct{},
	staged map[string]struct{},
	result *hostSkillSyncResult,
) error {
	targetRoot, err := root.OpenRootNoSymlink(targetParent)
	if err != nil {
		return fmt.Errorf("读取 %s 视图失败: %w", view, err)
	}
	entries, exceeded, readErr := readBoundedHostSkillDirectory(targetRoot, hostSkillMaxEntries)
	closeErr := targetRoot.Close()
	if readErr != nil || closeErr != nil {
		if readErr == nil {
			readErr = closeErr
		}
		return fmt.Errorf("枚举 %s 视图失败: %w", view, readErr)
	}
	if exceeded {
		return fmt.Errorf("%s 视图条目数量超过 %d", view, hostSkillMaxEntries)
	}

	names := make([]string, 0, len(staged))
	for name := range staged {
		names = append(names, name)
	}
	sort.Strings(names)
	for index, name := range names {
		sourceRelative := filepath.ToSlash(filepath.Join(stagingParent, name))
		targetRelative := filepath.ToSlash(filepath.Join(targetParent, name))
		backupRelative := filepath.ToSlash(filepath.Join(stagingParent, fmt.Sprintf(".previous-%d", index)))
		if err := replaceHostSkillEntry(root, sourceRelative, targetRelative, backupRelative); err != nil {
			result.skipped = append(result.skipped, skippedHostSkill{
				name:             name,
				err:              fmt.Errorf("发布 %s 视图失败: %w", view, err),
				retainedLastGood: hostSkillEntryReady(root, targetRelative),
			})
		}
	}

	for _, entry := range entries {
		if _, keep := kept[entry.Name()]; keep {
			continue
		}
		targetRelative := filepath.ToSlash(filepath.Join(targetParent, entry.Name()))
		if err = root.RemoveAll(targetRelative); err != nil && !os.IsNotExist(err) {
			result.skipped = append(result.skipped, skippedHostSkill{
				name:             entry.Name(),
				err:              fmt.Errorf("清理 %s 过期入口失败: %w", view, err),
				retainedLastGood: hostSkillEntryReady(root, targetRelative),
			})
		}
	}
	return nil
}

func retainedHostSkillNames(
	root *confinedfs.Root,
	staged map[string]struct{},
	result *hostSkillSyncResult,
) map[string]struct{} {
	kept := make(map[string]struct{}, len(staged)+len(result.skipped))
	for name := range staged {
		kept[name] = struct{}{}
	}
	for index := range result.skipped {
		name := result.skipped[index].name
		if result.skipped[index].discardLastGood {
			continue
		}
		target := filepath.ToSlash(filepath.Join(".agents", "skills", name))
		result.skipped[index].retainedLastGood = hostSkillEntryReady(root, target)
		if result.skipped[index].retainedLastGood {
			kept[name] = struct{}{}
		}
	}
	return kept
}

// replaceHostSkillEntry 把旧版备份留在隐藏 staging 区。
//
// 不能复用整库发布的 sibling `.old` 策略：放在
// `.agents/skills` 下的备份会被 nxs/Claude 短暂识别为新 Skill。
func replaceHostSkillEntry(
	root *confinedfs.Root,
	sourceRelative string,
	targetRelative string,
	backupRelative string,
) error {
	if _, err := root.Lstat(targetRelative); os.IsNotExist(err) {
		return root.Rename(sourceRelative, targetRelative)
	} else if err != nil {
		return err
	}
	if err := root.RemoveAll(backupRelative); err != nil {
		return err
	}
	if err := root.Rename(targetRelative, backupRelative); err != nil {
		return err
	}
	if err := root.Rename(sourceRelative, targetRelative); err != nil {
		_ = root.Rename(backupRelative, targetRelative)
		return err
	}
	return root.RemoveAll(backupRelative)
}

func hostSkillEntryReady(root *confinedfs.Root, relativePath string) bool {
	skillRoot, err := root.OpenRootNoSymlink(relativePath)
	if err != nil {
		return false
	}
	defer skillRoot.Close()
	file, err := skillRoot.OpenFileNoSymlink("SKILL.md", os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	return statErr == nil && closeErr == nil && info.Mode().IsRegular()
}

func syncClaudeHostSkillFallback(
	root *confinedfs.Root,
	result *hostSkillSyncResult,
) error {
	agentsRoot, err := root.OpenRootNoSymlink(".agents/skills")
	if err != nil {
		return err
	}
	defer agentsRoot.Close()
	stagingRelative, err := root.MkdirTemp(".", ".host-claude-staging-", 0o755)
	if err != nil {
		return err
	}
	defer root.RemoveAll(stagingRelative)
	stagingRoot, err := root.OpenRootNoSymlink(stagingRelative)
	if err != nil {
		return err
	}
	entries, exceeded, err := readBoundedHostSkillDirectory(agentsRoot, hostSkillMaxEntries)
	if err != nil {
		stagingRoot.Close()
		return err
	}
	if exceeded {
		stagingRoot.Close()
		return fmt.Errorf("canonical 视图条目数量超过 %d", hostSkillMaxEntries)
	}
	kept := make(map[string]struct{}, len(entries))
	staged := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		canonicalRelative := filepath.ToSlash(filepath.Join(".agents/skills", name))
		fallbackRelative := filepath.ToSlash(filepath.Join(".claude/skills", name))
		canonicalReady := hostSkillEntryReady(root, canonicalRelative)
		sourceChild, openErr := agentsRoot.OpenRootNoSymlink(name)
		if openErr != nil {
			retainedLastGood := canonicalReady && hostSkillEntryReady(root, fallbackRelative)
			if retainedLastGood {
				kept[name] = struct{}{}
			}
			result.skipped = append(result.skipped, skippedHostSkill{
				name:             name,
				err:              fmt.Errorf("读取 canonical Skill 失败: %w", openErr),
				retainedLastGood: retainedLastGood,
			})
			continue
		}
		targetChild, createErr := stagingRoot.OpenOrCreateRootNoSymlink(name, 0o755)
		if createErr != nil {
			sourceChild.Close()
			retainedLastGood := canonicalReady && hostSkillEntryReady(root, fallbackRelative)
			if retainedLastGood {
				kept[name] = struct{}{}
			}
			result.skipped = append(result.skipped, skippedHostSkill{
				name:             name,
				err:              fmt.Errorf("创建 Claude fallback staging 失败: %w", createErr),
				retainedLastGood: retainedLastGood,
			})
			continue
		}
		copyErr := copyRuntimeReadableSkillTree(targetChild, sourceChild)
		sourceCloseErr := sourceChild.Close()
		targetCloseErr := targetChild.Close()
		if copyErr == nil {
			copyErr = sourceCloseErr
		}
		if copyErr == nil {
			copyErr = targetCloseErr
		}
		if copyErr != nil {
			_ = stagingRoot.RemoveAll(name)
			retainedLastGood := canonicalReady && hostSkillEntryReady(root, fallbackRelative)
			if retainedLastGood {
				kept[name] = struct{}{}
			}
			result.skipped = append(result.skipped, skippedHostSkill{
				name:             name,
				err:              fmt.Errorf("构建 Claude fallback 失败: %w", copyErr),
				retainedLastGood: retainedLastGood,
			})
			continue
		}
		staged[name] = struct{}{}
		kept[name] = struct{}{}
	}
	if err = stagingRoot.Close(); err != nil {
		return err
	}
	return reconcileHostSkillEntries(
		root,
		stagingRelative,
		".claude/skills",
		"Claude fallback",
		kept,
		staged,
		result,
	)
}
