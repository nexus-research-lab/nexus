// INPUT: 已复制到目标 users 根的 Room 状态，以及 Agent 新旧 workspace 路径映射。
// OUTPUT: 只改写宿主结构字段中的绝对 workspace 路径，不触碰对话正文与工具输出。
// POS: users 根迁移的持久化元数据重映射阶段。
package userroot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

type workspacePathRewrite struct {
	oldRoot string
	newRoot string
}

func rewriteRoomWorkspacePaths(
	ctx context.Context,
	sourceRoot string,
	targetRoot string,
	records []agentWorkspaceRecord,
) error {
	byOwner := make(map[string][]workspacePathRewrite)
	for _, record := range records {
		oldPath := resolveAgentPathAt(sourceRoot, record)
		newPath := resolveAgentPathAt(targetRoot, record)
		if samePath(oldPath, newPath) {
			continue
		}
		byOwner[record.ownerUserID] = append(
			byOwner[record.ownerUserID],
			workspacePathRewrite{oldRoot: oldPath, newRoot: newPath},
		)
	}
	for ownerUserID, rewrites := range byOwner {
		roomRoot := appfs.UserRoomRootAtUsersRoot(targetRoot, ownerUserID)
		if err := rewriteRoomStateTree(ctx, roomRoot, rewrites); err != nil {
			return err
		}
	}
	return nil
}

func rewriteRoomStateTree(
	ctx context.Context,
	roomRoot string,
	rewrites []workspacePathRewrite,
) error {
	root, err := confinedfs.Open(roomRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		info, err := root.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".json":
			return rewriteJSONPathFile(root, path, info.Mode().Perm(), rewrites)
		case ".jsonl":
			return rewriteJSONLPathFile(root, path, info.Mode().Perm(), rewrites)
		default:
			return nil
		}
	})
}

func rewriteJSONPathFile(
	root *confinedfs.Root,
	path string,
	mode os.FileMode,
	rewrites []workspacePathRewrite,
) error {
	payload, err := root.ReadFile(path)
	if err != nil {
		return err
	}
	var value any
	if err = json.Unmarshal(payload, &value); err != nil {
		// 历史脏文件由原有读取链路处理；迁移不能因无关坏行丢弃整个 users 根。
		return nil
	}
	if !rewriteStructuredWorkspacePaths(value, rewrites) {
		return nil
	}
	next, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	next = append(next, '\n')
	return root.WriteFileAtomic(path, next, mode)
}

func rewriteJSONLPathFile(
	root *confinedfs.Root,
	path string,
	mode os.FileMode,
	rewrites []workspacePathRewrite,
) error {
	payload, err := root.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(payload, []byte{'\n'})
	changed := false
	for index, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var value any
		if err = json.Unmarshal(line, &value); err != nil {
			continue
		}
		if !rewriteStructuredWorkspacePaths(value, rewrites) {
			continue
		}
		lines[index], err = json.Marshal(value)
		if err != nil {
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return root.WriteFileAtomic(path, bytes.Join(lines, []byte{'\n'}), mode)
}

func rewriteStructuredWorkspacePaths(value any, rewrites []workspacePathRewrite) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if isWorkspacePathField(key) {
				if path, ok := item.(string); ok {
					if rewritten, matched := rewriteWorkspacePath(path, rewrites); matched {
						typed[key] = rewritten
						changed = true
					}
				}
			}
			if rewriteStructuredWorkspacePaths(typed[key], rewrites) {
				changed = true
			}
		}
	case []any:
		for _, item := range typed {
			if rewriteStructuredWorkspacePaths(item, rewrites) {
				changed = true
			}
		}
	}
	return changed
}

func isWorkspacePathField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "workspace_path", "cwd", "project_path":
		return true
	default:
		return false
	}
}

func rewriteWorkspacePath(path string, rewrites []workspacePathRewrite) (string, bool) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return path, false
	}
	for _, rewrite := range rewrites {
		if samePath(path, rewrite.oldRoot) {
			return rewrite.newRoot, true
		}
		if !pathContains(rewrite.oldRoot, path) {
			continue
		}
		relative, err := filepath.Rel(rewrite.oldRoot, path)
		if err != nil {
			continue
		}
		return filepath.Join(rewrite.newRoot, relative), true
	}
	return path, false
}
