package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// listRoomOwnerPathSegments 返回已经落盘的用户路径段，供进程恢复任务逐用户扫描。
func listRoomOwnerPathSegments(stateRoot string) ([]string, error) {
	root, err := openManagedSubtree(stateRoot, filepath.Join(stateRoot, "users"), false, 0)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	owners := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			owners = append(owners, entry.Name())
		}
	}
	sort.Strings(owners)
	return owners, nil
}

// roomLedgerOwnerUserID 只接受与扫描目录一致的持久化 owner。
//
// owner 路径段是安全边界，但它不是业务身份；恢复任务必须保留 JSONL 中
// 的原始 owner，并用同一套路径映射证明它确实属于当前目录。
func roomLedgerOwnerUserID(ownerPathSegment string, persistedOwnerUserID string) (string, bool) {
	ownerPathSegment = strings.TrimSpace(ownerPathSegment)
	persistedOwnerUserID = strings.TrimSpace(persistedOwnerUserID)
	if ownerPathSegment == "" || persistedOwnerUserID == "" {
		return "", false
	}
	if appfs.UserPathSegment(persistedOwnerUserID) != ownerPathSegment {
		return "", false
	}
	return persistedOwnerUserID, true
}
