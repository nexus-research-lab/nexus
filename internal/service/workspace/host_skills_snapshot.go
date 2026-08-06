// INPUT: 已固定的宿主 Skill 源目录与单项复制预算。
// OUTPUT: 经过直接目录、链接边界和普通文件校验的 staging 快照。
// POS: 宿主 Skill 来源到受管投影之间的安全复制层。
package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const (
	hostSkillCopyMaxDepth   = 32
	hostSkillMaxDirectories = 2_000
	hostSkillMaxEntries     = 20_000
	hostSkillMaxFileBytes   = int64(64 << 20)
	hostSkillMaxTotalBytes  = int64(512 << 20)
)

type hostSkillCopyStats struct {
	directories int
	entries     int
	bytes       int64
}

var errHostSkillCopyLimit = errors.New("宿主 Skill 文件超过复制预算")

type hostSkillBoundedReader struct {
	source    io.Reader
	remaining int64
	copied    int64
}

func (r *hostSkillBoundedReader) Read(buffer []byte) (int, error) {
	if int64(len(buffer)) > r.remaining+1 {
		buffer = buffer[:r.remaining+1]
	}
	read, err := r.source.Read(buffer)
	r.copied += int64(read)
	r.remaining -= int64(read)
	if r.remaining < 0 {
		return read, errHostSkillCopyLimit
	}
	return read, err
}

type hostSkillSnapshotBuilder struct {
	home             string
	sourcePath       string
	source           *confinedfs.Root
	total            hostSkillCopyStats
	staged           map[string]struct{}
	canonicalNames   map[string]string
	watchDirectories map[string]struct{}
	result           hostSkillSyncResult
}

func (b *hostSkillSnapshotBuilder) buildAgentsView(target *confinedfs.Root) error {
	entries, exceeded, err := readBoundedHostSkillDirectory(b.source, hostSkillMaxEntries)
	if err != nil {
		return err
	}
	if exceeded {
		return fmt.Errorf("宿主 Skill 顶层条目数量超过 %d", hostSkillMaxEntries)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if nameErr := validateHostSkillEntryName(name); nameErr != nil {
			b.result.skipped = append(b.result.skipped, skippedHostSkill{
				name:            name,
				err:             nameErr,
				discardLastGood: true,
			})
			continue
		}
		canonicalKey := strings.ToLower(name)
		if existing, duplicate := b.canonicalNames[canonicalKey]; duplicate {
			b.result.skipped = append(b.result.skipped, skippedHostSkill{
				name:            name,
				err:             fmt.Errorf("宿主 Skill 目录名与 %s 仅大小写不同", existing),
				discardLastGood: true,
			})
			continue
		}
		b.canonicalNames[canonicalKey] = name
		if copyErr := b.copySourceEntry(target, name); copyErr != nil {
			_ = target.RemoveAll(name)
			b.result.skipped = append(b.result.skipped, skippedHostSkill{
				name: name,
				err:  copyErr,
			})
			continue
		}
		b.staged[name] = struct{}{}
	}
	return nil
}

func validateHostSkillEntryName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed != name {
		return errors.New("宿主 Skill 目录名不能为空或包含首尾空白")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "external:") {
		return errors.New("宿主 Skill 目录名不能使用 external: 保留前缀")
	}
	return nil
}

func (b *hostSkillSnapshotBuilder) copySourceEntry(
	target *confinedfs.Root,
	name string,
) error {
	info, err := b.source.Lstat(name)
	if err != nil {
		return err
	}
	entryPath := filepath.Join(b.sourcePath, name)
	linkedDirectory, err := isHostSkillDirectoryLink(entryPath, info)
	if err != nil {
		return err
	}

	var sourceChild *confinedfs.Root
	resolvedPath := entryPath
	if linkedDirectory {
		sourceChild, resolvedPath, err = openHostSkillDirectoryLink(b.home, entryPath)
	} else if !info.IsDir() {
		return errors.New("宿主 Skill 根只接受目录项")
	} else {
		sourceChild, err = b.source.OpenRootNoSymlink(name)
	}
	if err != nil {
		return err
	}
	b.watchDirectories[resolvedPath] = struct{}{}
	if err = validateHostSkillDirectory(sourceChild); err != nil {
		sourceChild.Close()
		return err
	}

	targetChild, err := target.OpenOrCreateRootNoSymlink(name, 0o755)
	if err != nil {
		sourceChild.Close()
		return err
	}
	stats := b.total
	watchDirectories := make(map[string]struct{})
	copyErr := copyBoundedHostSkillTree(
		targetChild,
		sourceChild,
		resolvedPath,
		0,
		&stats,
		watchDirectories,
	)
	// 即使当前 Skill 最终被拒绝，已经消耗的目录、条目与字节也属于本轮预算。
	// 否则大量“末尾才失败”的 Skill 可以反复重置计数，绕过整轮上限。
	b.total = stats
	for directory := range watchDirectories {
		b.watchDirectories[directory] = struct{}{}
	}
	sourceCloseErr := sourceChild.Close()
	targetCloseErr := targetChild.Close()
	if copyErr != nil {
		return copyErr
	}
	if sourceCloseErr != nil {
		return sourceCloseErr
	}
	if targetCloseErr != nil {
		return targetCloseErr
	}
	return nil
}

func validateHostSkillDirectory(root *confinedfs.Root) error {
	file, err := root.OpenFileNoSymlink("SKILL.md", os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("目录不是直接 Skill：缺少真实 SKILL.md: %w", err)
	}
	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil {
		return statErr
	}
	if !info.Mode().IsRegular() {
		return errors.New("SKILL.md 不是普通文件")
	}
	if info.Size() > hostSkillMaxFileBytes {
		return fmt.Errorf("SKILL.md 超过 %d MiB", hostSkillMaxFileBytes>>20)
	}
	return closeErr
}

func (b *hostSkillSnapshotBuilder) syncResult() hostSkillSyncResult {
	b.result.watchDirectories = sortedHostSkillWatchDirectories(b.watchDirectories)
	return b.result
}

func copyBoundedHostSkillTree(
	target *confinedfs.Root,
	source *confinedfs.Root,
	sourcePath string,
	depth int,
	stats *hostSkillCopyStats,
	watchDirectories map[string]struct{},
) error {
	if depth > hostSkillCopyMaxDepth {
		return fmt.Errorf("Skill 目录深度超过 %d 层", hostSkillCopyMaxDepth)
	}
	stats.directories++
	if err := validateHostSkillCopyStats(*stats); err != nil {
		return err
	}
	if watchDirectories != nil && sourcePath != "" {
		watchDirectories[filepath.Clean(sourcePath)] = struct{}{}
	}
	remainingEntries := hostSkillMaxEntries - stats.entries
	entries, exceeded, err := readBoundedHostSkillDirectory(source, remainingEntries)
	if err != nil {
		return err
	}
	if exceeded {
		return fmt.Errorf("宿主 Skill 条目数量超过 %d", hostSkillMaxEntries)
	}
	for _, entry := range entries {
		name := entry.Name()
		stats.entries++
		if err = validateHostSkillCopyStats(*stats); err != nil {
			return err
		}
		info, err := source.Lstat(name)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Skill 内部不允许链接: %s", name)
		}
		if info.IsDir() {
			sourceChild, err := source.OpenRootNoSymlink(name)
			if err != nil {
				return err
			}
			targetChild, createErr := target.OpenOrCreateRootNoSymlink(name, 0o755)
			if createErr != nil {
				sourceChild.Close()
				return createErr
			}
			childPath := ""
			if sourcePath != "" {
				childPath = filepath.Join(sourcePath, name)
			}
			copyErr := copyBoundedHostSkillTree(
				targetChild,
				sourceChild,
				childPath,
				depth+1,
				stats,
				watchDirectories,
			)
			sourceCloseErr := sourceChild.Close()
			targetCloseErr := targetChild.Close()
			if copyErr != nil {
				return copyErr
			}
			if sourceCloseErr != nil {
				return sourceCloseErr
			}
			if targetCloseErr != nil {
				return targetCloseErr
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Skill 包含特殊文件: %s", name)
		}
		sourceFile, err := source.OpenFileNoSymlink(name, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		openedInfo, err := sourceFile.Stat()
		if err != nil {
			sourceFile.Close()
			return err
		}
		if !openedInfo.Mode().IsRegular() {
			sourceFile.Close()
			return fmt.Errorf("Skill 包含特殊文件: %s", name)
		}
		if openedInfo.Size() > hostSkillMaxFileBytes {
			sourceFile.Close()
			return fmt.Errorf("Skill 文件超过 %d MiB: %s", hostSkillMaxFileBytes>>20, name)
		}
		remainingTotalBytes := hostSkillMaxTotalBytes - stats.bytes
		if openedInfo.Size() > remainingTotalBytes {
			sourceFile.Close()
			return fmt.Errorf("宿主 Skill 总大小超过 %d MiB", hostSkillMaxTotalBytes>>20)
		}
		mode := os.FileMode(0o644)
		if openedInfo.Mode().Perm()&0o111 != 0 {
			mode = 0o755
		}
		copyLimit := min(hostSkillMaxFileBytes, remainingTotalBytes)
		boundedSource := &hostSkillBoundedReader{
			source:    sourceFile,
			remaining: copyLimit,
		}
		copyErr := target.WriteFileAtomicFrom(name, boundedSource, mode)
		closeErr := sourceFile.Close()
		stats.bytes += boundedSource.copied
		if copyErr != nil {
			return fmt.Errorf("复制 Skill 文件 %s 失败: %w", name, copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return target.ChmodRoot(0o755)
}

// readBoundedHostSkillDirectory 在分配完整目录列表前截断枚举。
//
// fs.ReadDir 会先把全部目录项读入内存，再由调用方判断长度；宿主目录属于
// 外部输入，必须把条目预算落实到系统调用这一层。
func readBoundedHostSkillDirectory(
	root *confinedfs.Root,
	limit int,
) ([]fs.DirEntry, bool, error) {
	if limit < 0 {
		return nil, true, nil
	}
	directory, err := root.Open(".")
	if err != nil {
		return nil, false, err
	}
	entries, readErr := directory.ReadDir(limit + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, false, readErr
	}
	if closeErr != nil {
		return nil, false, closeErr
	}
	if len(entries) > limit {
		return nil, true, nil
	}
	sort.Slice(entries, func(left int, right int) bool {
		return entries[left].Name() < entries[right].Name()
	})
	return entries, false, nil
}

func validateHostSkillCopyStats(stats hostSkillCopyStats) error {
	switch {
	case stats.directories > hostSkillMaxDirectories:
		return fmt.Errorf("宿主 Skill 目录数量超过 %d", hostSkillMaxDirectories)
	case stats.entries > hostSkillMaxEntries:
		return fmt.Errorf("宿主 Skill 条目数量超过 %d", hostSkillMaxEntries)
	case stats.bytes > hostSkillMaxTotalBytes:
		return fmt.Errorf("宿主 Skill 总大小超过 %d MiB", hostSkillMaxTotalBytes>>20)
	default:
		return nil
	}
}

// openHostSkillDirectoryLink 只授权用户 home 内的显式顶层 Skill 目标。
//
// 解析完成后从固定的 home fd 逐级打开真实目录；后续复制不会再次跟随原
// junction，也不会允许目标树内部的二次链接。
func openHostSkillDirectoryLink(
	home string,
	linkPath string,
) (*confinedfs.Root, string, error) {
	resolvedTarget, err := filepath.EvalSymlinks(filepath.Clean(linkPath))
	if err != nil {
		return nil, "", fmt.Errorf("解析宿主 Skill 链接失败: %w", err)
	}
	if err = validateHostSkillProjectionBoundary(resolvedTarget); err != nil {
		return nil, "", err
	}
	relativeTarget, err := relativePathWithin(home, resolvedTarget)
	if err != nil {
		return nil, "", fmt.Errorf("宿主 Skill 链接目标必须位于用户目录内: %w", err)
	}
	homeRoot, err := confinedfs.Open(home)
	if err != nil {
		return nil, "", err
	}
	defer homeRoot.Close()
	linkedRoot, err := homeRoot.OpenRootNoSymlink(relativeTarget)
	if err != nil {
		return nil, "", fmt.Errorf("打开宿主 Skill 链接目标失败: %w", err)
	}
	return linkedRoot, resolvedTarget, nil
}
