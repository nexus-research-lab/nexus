package workspace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// OpenOwnerWorkspacePath 从配置根逐级打开 owner workspace 或 Agent 子目录。
//
// 绝对路径只用于确认结构归属；真正的目录解析从 state/workspace 根的固定
// fd 进入，拒绝 owner、workspace 或 Agent 任一层被替换成符号链接。
func (s *Store) OpenOwnerWorkspacePath(
	ownerUserID string,
	workspacePath string,
	create bool,
) (*confinedfs.Root, error) {
	workspacePath = filepath.Clean(strings.TrimSpace(workspacePath))
	if workspacePath == "" || workspacePath == "." {
		return nil, errors.New("workspace path is empty")
	}
	canonicalOwnerRoot := appfs.UserWorkspaceRootAt(s.StateRoot, ownerUserID)
	if sameManagedWorkspacePath(canonicalOwnerRoot, workspacePath) {
		return openManagedSubtree(
			s.StateRoot,
			workspacePath,
			create,
			storageDirectoryMode(),
		)
	}
	configuredOwnerRoot := filepath.Join(
		s.WorkspaceRoot,
		appfs.UserPathSegment(ownerUserID),
		"workspace",
	)
	if sameManagedWorkspacePath(configuredOwnerRoot, workspacePath) {
		return openManagedSubtree(
			s.WorkspaceRoot,
			workspacePath,
			create,
			storageDirectoryMode(),
		)
	}
	return s.openWorkspacePathForOwner(ownerUserID, workspacePath, create)
}

// RemoveOwnerWorkspacePath 从固定的 owner workspace 父目录删除一个 Agent 目录。
//
// 删除只作用于最后一个目录项；即使该项已被替换成 symlink，也只会删除链接
// 本身，不会递归进入另一用户的目录。
func (s *Store) RemoveOwnerWorkspacePath(
	ownerUserID string,
	workspacePath string,
) error {
	workspacePath = filepath.Clean(strings.TrimSpace(workspacePath))
	canonicalOwnerRoot := appfs.UserWorkspaceRootAt(s.StateRoot, ownerUserID)
	if directChildPath(canonicalOwnerRoot, workspacePath) {
		return removeManagedWorkspaceChild(
			s.StateRoot,
			canonicalOwnerRoot,
			workspacePath,
		)
	}
	configuredOwnerRoot := filepath.Join(
		s.WorkspaceRoot,
		appfs.UserPathSegment(ownerUserID),
		"workspace",
	)
	if directChildPath(configuredOwnerRoot, workspacePath) {
		return removeManagedWorkspaceChild(
			s.WorkspaceRoot,
			configuredOwnerRoot,
			workspacePath,
		)
	}
	return errors.New("workspace path is not an owner Agent directory")
}

func removeManagedWorkspaceChild(
	managedRoot string,
	ownerWorkspaceRoot string,
	workspacePath string,
) error {
	ownerRoot, err := openManagedSubtree(
		managedRoot,
		ownerWorkspaceRoot,
		false,
		storageDirectoryMode(),
	)
	if err != nil {
		return err
	}
	defer ownerRoot.Close()
	return ownerRoot.RemoveAll(filepath.Base(workspacePath))
}

func sameManagedWorkspacePath(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (s *Store) openWorkspacePathForOwner(
	ownerUserID string,
	workspacePath string,
	create bool,
) (*confinedfs.Root, error) {
	workspacePath = filepath.Clean(strings.TrimSpace(workspacePath))
	if !s.workspacePathBelongsToOwner(ownerUserID, workspacePath) {
		return nil, errors.New("workspace path does not belong to owner")
	}
	canonicalOwnerRoot := appfs.UserWorkspaceRootAt(s.StateRoot, ownerUserID)
	if directChildPath(canonicalOwnerRoot, workspacePath) {
		return openManagedSubtree(
			s.StateRoot,
			workspacePath,
			create,
			storageDirectoryMode(),
		)
	}
	configuredOwnerRoot := filepath.Join(
		s.WorkspaceRoot,
		appfs.UserPathSegment(ownerUserID),
		"workspace",
	)
	if directChildPath(configuredOwnerRoot, workspacePath) {
		return openManagedSubtree(
			s.WorkspaceRoot,
			workspacePath,
			create,
			storageDirectoryMode(),
		)
	}
	return nil, errors.New("workspace path does not belong to a managed owner root")
}

func (s *Store) workspacePathIsConfinedForOwner(
	ownerUserID string,
	workspacePath string,
) bool {
	root, err := s.openWorkspacePathForOwner(ownerUserID, workspacePath, false)
	if errors.Is(err, os.ErrNotExist) {
		// workspace 已删除时仍允许按 owner 归属解析历史 transcript；不存在的
		// 路径不能承载 symlink，真正的 transcript 仍会在 runtime projects 根
		// 下再次经过 confinedfs 校验。
		return s.workspacePathBelongsToOwner(ownerUserID, workspacePath)
	}
	if err != nil {
		return false
	}
	root.Close()
	return true
}

// OpenOwnerWorkspaceFile 从 owner 的 Agent workspace 打开真实普通文件。
//
// 返回路径只用于投递给 runtime；宿主读取必须使用一并返回的文件描述符，
// 不能在校验后重新按绝对路径打开。
func (s *Store) OpenOwnerWorkspaceFile(
	ownerUserID string,
	workspacePath string,
	relativePath string,
) (string, *os.File, error) {
	root, err := s.openWorkspacePathForOwner(ownerUserID, workspacePath, false)
	if err != nil {
		return "", nil, err
	}
	defer root.Close()
	return openManagedRegularFile(root, workspacePath, relativePath)
}

// OpenRoomConversationAssetFile 从 owner 的 Room 公共资产根打开真实普通文件。
func (s *Store) OpenRoomConversationAssetFile(
	ownerUserID string,
	conversationID string,
	relativePath string,
) (string, *os.File, error) {
	workspaceRootPath := appfs.UserWorkspaceRootAt(s.StateRoot, ownerUserID)
	workspaceRoot, err := s.openOwnerWorkspaceRoot(ownerUserID, false)
	if err != nil {
		return "", nil, err
	}

	assetRootPath := s.RoomConversationAssetDir(ownerUserID, conversationID)
	assetRootRelative, err := relativeManagedPath(workspaceRootPath, assetRootPath)
	if err != nil {
		workspaceRoot.Close()
		return "", nil, err
	}
	assetRoot, err := workspaceRoot.OpenRootNoSymlink(assetRootRelative)
	workspaceRoot.Close()
	if err != nil {
		return "", nil, err
	}
	defer assetRoot.Close()
	return openManagedRegularFile(assetRoot, assetRootPath, relativePath)
}

func openManagedRegularFile(
	root *confinedfs.Root,
	rootPath string,
	relativePath string,
) (string, *os.File, error) {
	relativePath = strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/"))
	relativePath = strings.TrimPrefix(relativePath, "/")
	if relativePath == "" || path.IsAbs(relativePath) {
		return "", nil, errors.New("managed file path is empty or absolute")
	}
	relativePath = path.Clean(relativePath)
	if relativePath == "." || relativePath == ".." ||
		strings.HasPrefix(relativePath, "../") {
		return "", nil, errors.New("managed file path escapes root")
	}

	parent, err := root.OpenRootNoSymlink(path.Dir(relativePath))
	if err != nil {
		return "", nil, err
	}
	defer parent.Close()
	file, err := parent.OpenFileNoSymlink(path.Base(relativePath), os.O_RDONLY, 0)
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(rootPath, filepath.FromSlash(relativePath)), file, nil
}

func (s *SessionFileStore) openOwnerWorkspaceFileParent(
	ownerUserID string,
	workspacePath string,
	target string,
	create bool,
) (*confinedfs.Root, string, error) {
	if s == nil || s.paths == nil {
		return nil, "", errors.New("workspace storage root is nil")
	}
	relative, err := relativeManagedPath(workspacePath, target)
	if err != nil {
		return nil, "", err
	}
	if relative == "." {
		return nil, "", errors.New("workspace file path cannot be the workspace root")
	}
	root, err := s.paths.openWorkspacePathForOwner(ownerUserID, workspacePath, create)
	if err != nil {
		return nil, "", err
	}
	parentPath := path.Dir(relative)
	var parent *confinedfs.Root
	if create {
		parent, err = root.OpenOrCreateRootNoSymlink(parentPath, storageDirectoryMode())
	} else {
		parent, err = root.OpenRootNoSymlink(parentPath)
	}
	root.Close()
	if err != nil {
		return nil, "", err
	}
	return parent, path.Base(relative), nil
}

func (s *SessionFileStore) appendOwnerWorkspaceJSONL(
	ownerUserID string,
	workspacePath string,
	target string,
	row map[string]any,
) error {
	parent, name, err := s.openOwnerWorkspaceFileParent(
		ownerUserID,
		workspacePath,
		target,
		true,
	)
	if err != nil {
		return err
	}
	defer parent.Close()
	return appendJSONLAtRoot(parent, name, row)
}

func (s *SessionFileStore) readOwnerWorkspaceJSONL(
	ownerUserID string,
	workspacePath string,
	target string,
) ([]map[string]any, error) {
	parent, name, err := s.openOwnerWorkspaceFileParent(
		ownerUserID,
		workspacePath,
		target,
		false,
	)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return readJSONLAtRoot(parent, name)
}

func (s *SessionFileStore) replaceOwnerWorkspaceJSONL(
	ownerUserID string,
	workspacePath string,
	target string,
	rows []map[string]any,
) error {
	parent, name, err := s.openOwnerWorkspaceFileParent(
		ownerUserID,
		workspacePath,
		target,
		true,
	)
	if err != nil {
		return err
	}
	defer parent.Close()

	var builder strings.Builder
	writer := bufio.NewWriter(&builder)
	for _, row := range rows {
		payload, marshalErr := json.Marshal(row)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := fmt.Fprintf(writer, "%s\n", payload); writeErr != nil {
			return writeErr
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}
	return parent.WriteFileAtomic(name, []byte(builder.String()), storageFileMode(0o644))
}
