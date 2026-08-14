package workspace

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// openManagedSubtree 从固定的宿主管理根逐级打开子目录。
//
// managedRoot 是宿主确认过的配置边界；其下的 owner 与业务目录都必须是
// 真实目录，不能通过同一状态树内的符号链接借用另一用户的 inode。
func openManagedSubtree(
	managedRoot string,
	target string,
	create bool,
	perm os.FileMode,
) (*confinedfs.Root, error) {
	managedRoot = filepath.Clean(strings.TrimSpace(managedRoot))
	if managedRoot == "" || managedRoot == "." {
		return nil, errors.New("managed storage root is empty")
	}
	relative, err := relativeManagedPath(managedRoot, target)
	if err != nil {
		return nil, err
	}
	if create {
		// 管理根来自宿主配置，可以在进入逐级 no-symlink 边界前创建。
		if err = os.MkdirAll(managedRoot, 0o700); err != nil {
			return nil, err
		}
	}
	root, err := confinedfs.Open(managedRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if create {
		return root.OpenOrCreateRootNoSymlink(relative, perm)
	}
	return root.OpenRootNoSymlink(relative)
}

func relativeManagedPath(rootPath string, target string) (string, error) {
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	target = filepath.Clean(strings.TrimSpace(target))
	if rootPath == "" || rootPath == "." || target == "" || target == "." {
		return "", errors.New("managed storage path is empty")
	}
	relative, err := filepath.Rel(rootPath, target)
	if err != nil ||
		relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("managed storage path outside configured root")
	}
	relative = filepath.ToSlash(relative)
	if relative == "" {
		return ".", nil
	}
	return relative, nil
}

func (s *Store) openOwnerWorkspaceRoot(
	ownerUserID string,
	create bool,
) (*confinedfs.Root, error) {
	if s == nil {
		return nil, errors.New("workspace path store is nil")
	}
	return openManagedSubtree(
		s.StateRoot,
		appfs.UserWorkspaceRootAt(s.StateRoot, ownerUserID),
		create,
		storageDirectoryMode(),
	)
}

func (s *Store) openOwnerTranscriptProjectsRoot(
	ownerUserID string,
	create bool,
) (*confinedfs.Root, error) {
	if s == nil {
		return nil, errors.New("workspace path store is nil")
	}
	return openManagedSubtree(
		s.StateRoot,
		filepath.Join(
			appfs.UserRuntimeRootAt(s.StateRoot, ownerUserID),
			"projects",
		),
		create,
		storageDirectoryMode(),
	)
}

func (s *SessionFileStore) openRoomRoot(
	ownerUserID string,
	create bool,
) (*confinedfs.Root, error) {
	if s == nil || s.paths == nil {
		return nil, errors.New("workspace storage root is nil")
	}
	return openManagedSubtree(
		s.paths.StateRoot,
		s.paths.RoomConversationRoot(ownerUserID),
		create,
		storagePrivateDirectoryMode(),
	)
}

func (s *SessionFileStore) openRoomFileParent(
	ownerUserID string,
	target string,
	create bool,
) (*confinedfs.Root, string, error) {
	roomRootPath := s.paths.RoomConversationRoot(ownerUserID)
	relative, err := relativeManagedPath(roomRootPath, target)
	if err != nil {
		return nil, "", err
	}
	if relative == "." {
		return nil, "", errors.New("Room ledger path cannot be the Room root")
	}
	root, err := s.openRoomRoot(ownerUserID, create)
	if err != nil {
		return nil, "", err
	}
	parentPath := path.Dir(relative)
	var parent *confinedfs.Root
	if create {
		parent, err = root.OpenOrCreateRootNoSymlink(parentPath, storagePrivateDirectoryMode())
	} else {
		parent, err = root.OpenRootNoSymlink(parentPath)
	}
	root.Close()
	if err != nil {
		return nil, "", err
	}
	return parent, path.Base(relative), nil
}

func (s *SessionFileStore) appendRoomJSONL(
	ownerUserID string,
	target string,
	row map[string]any,
) error {
	parent, name, err := s.openRoomFileParent(ownerUserID, target, true)
	if err != nil {
		return err
	}
	defer parent.Close()
	return appendJSONLAtRootWithMode(parent, name, row, storageFileMode(0o600))
}

func (s *SessionFileStore) readRoomJSONL(
	ownerUserID string,
	target string,
) ([]map[string]any, error) {
	parent, name, err := s.openRoomFileParent(ownerUserID, target, false)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return readJSONLAtRoot(parent, name)
}
