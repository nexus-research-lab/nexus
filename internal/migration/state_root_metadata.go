// INPUT: 新状态根内的 owner Room、Agent session JSON/JSONL 与新旧状态根映射。
// OUTPUT: 只改写结构字段中的受管绝对路径，不触碰消息正文和工具输出。
// POS: 桌面状态根启动迁移的文件元数据阶段。
package migration

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

func rewriteManagedStateRootPaths(
	ctx context.Context,
	currentRoot string,
	previousRoot string,
	targetRoot string,
) error {
	usersRoot := filepath.Join(currentRoot, "users")
	users, err := confinedfs.Open(usersRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer users.Close()
	owners, err := fs.ReadDir(users.FS(), ".")
	if err != nil {
		return err
	}
	for _, owner := range owners {
		if err = ctx.Err(); err != nil {
			return err
		}
		if !owner.IsDir() || owner.Type()&os.ModeSymlink != 0 {
			continue
		}
		roomPath := filepath.ToSlash(filepath.Join(owner.Name(), "state", "rooms"))
		if err = rewriteManagedTreeAt(ctx, users, roomPath, previousRoot, targetRoot); err != nil {
			return err
		}
		if err = rewriteOwnerAgentSessionPaths(
			ctx,
			users,
			owner.Name(),
			previousRoot,
			targetRoot,
		); err != nil {
			return err
		}
	}
	return nil
}

func rewriteOwnerAgentSessionPaths(
	ctx context.Context,
	users *confinedfs.Root,
	ownerName string,
	previousRoot string,
	targetRoot string,
) error {
	workspacePath := filepath.ToSlash(filepath.Join(ownerName, "workspace"))
	workspace, err := users.OpenRootNoSymlink(workspacePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer workspace.Close()
	agents, err := fs.ReadDir(workspace.FS(), ".")
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if err = ctx.Err(); err != nil {
			return err
		}
		if !agent.IsDir() || agent.Type()&os.ModeSymlink != 0 {
			continue
		}
		sessionsPath := filepath.ToSlash(filepath.Join(agent.Name(), ".agents", "sessions"))
		if err = rewriteManagedTreeAt(
			ctx,
			workspace,
			sessionsPath,
			previousRoot,
			targetRoot,
		); err != nil {
			return err
		}
	}
	return nil
}

func rewriteManagedTreeAt(
	ctx context.Context,
	parent *confinedfs.Root,
	relative string,
	previousRoot string,
	targetRoot string,
) error {
	root, err := parent.OpenRootNoSymlink(relative)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	return rewriteManagedStateTree(ctx, root, previousRoot, targetRoot)
}

func rewriteManagedStateTree(
	ctx context.Context,
	root *confinedfs.Root,
	previousRoot string,
	targetRoot string,
) error {
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
		if entry.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".json":
			return rewriteStateRootJSON(root, path, info.Mode().Perm(), previousRoot, targetRoot)
		case ".jsonl":
			return rewriteStateRootJSONL(root, path, info.Mode().Perm(), previousRoot, targetRoot)
		default:
			return nil
		}
	})
}

func rewriteStateRootJSON(
	root *confinedfs.Root,
	path string,
	mode os.FileMode,
	previousRoot string,
	targetRoot string,
) error {
	payload, err := root.ReadFile(path)
	if err != nil {
		return err
	}
	var value any
	if err = json.Unmarshal(payload, &value); err != nil {
		// 历史脏文件由原读取链路处理，迁移不能因无关坏文件阻断整根切换。
		return nil
	}
	if !rewriteStructuredStateRootPaths(value, previousRoot, targetRoot) {
		return nil
	}
	next, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFileAtomic(path, append(next, '\n'), mode)
}

func rewriteStateRootJSONL(
	root *confinedfs.Root,
	path string,
	mode os.FileMode,
	previousRoot string,
	targetRoot string,
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
		if !rewriteStructuredStateRootPaths(value, previousRoot, targetRoot) {
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

func rewriteStructuredStateRootPaths(value any, previousRoot string, targetRoot string) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if isStateRootPathField(key) {
				if path, ok := item.(string); ok {
					if rewritten, matched := appfs.RebaseStateRootPath(
						path,
						previousRoot,
						targetRoot,
					); matched {
						typed[key] = rewritten
						changed = true
					}
				}
			}
			if rewriteStructuredStateRootPaths(typed[key], previousRoot, targetRoot) {
				changed = true
			}
		}
	case []any:
		for _, item := range typed {
			if rewriteStructuredStateRootPaths(item, previousRoot, targetRoot) {
				changed = true
			}
		}
	}
	return changed
}

func isStateRootPathField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "workspace_path", "cwd", "project_path", "transcript_path", "artifact_path", "output_file":
		return true
	default:
		return false
	}
}
