// INPUT: 会话目录下单个 session 子目录名、meta.json 的 lstat 结果与上次安全读取的原始字节。
// OUTPUT: 文件身份、大小与修改时间均未变化时复用的 meta 原始字节，否则经 no-symlink 路径重新读取。
// POS: SessionFileStore.ListSessions 的进程内读缓存；不参与写入，也不改变任何读取失败的语义。
package workspace

import (
	"errors"
	"os"
	"path"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// sessionMetaSnapshot 只缓存已经安全读取并成功解码的原始字节；每次命中仍重新解码，
// 调用方拿到的 Session 与 Options map 互不共享。
type sessionMetaSnapshot struct {
	identity       string
	size           int64
	modifiedUnixNS int64
	payload        []byte
}

var sessionMetaSnapshots = struct {
	sync.Mutex
	byWorkspace map[string]map[string]sessionMetaSnapshot
}{byWorkspace: map[string]map[string]sessionMetaSnapshot{}}

func loadSessionMetaSnapshots(workspaceRoot string) map[string]sessionMetaSnapshot {
	sessionMetaSnapshots.Lock()
	defer sessionMetaSnapshots.Unlock()
	return sessionMetaSnapshots.byWorkspace[workspaceRoot]
}

// storeSessionMetaSnapshots 以本次完整列表替换整组快照，已删除的会话随之淘汰。
func storeSessionMetaSnapshots(workspaceRoot string, snapshots map[string]sessionMetaSnapshot) {
	sessionMetaSnapshots.Lock()
	defer sessionMetaSnapshots.Unlock()
	sessionMetaSnapshots.byWorkspace[workspaceRoot] = snapshots
}

// readListedSessionMeta 先用一次 lstat 判断 meta.json 是否仍是上次读取的那个文件；
// 原子写入会换新 inode，原地改写会改变大小或修改时间，任一不符都回到 no-symlink 读取。
// 拿不到稳定文件身份的平台不缓存。ok=false 表示该目录项不是可列出的会话。
func readListedSessionMeta(
	sessionsRoot *confinedfs.Root,
	name string,
	previous sessionMetaSnapshot,
) (protocol.Session, sessionMetaSnapshot, bool, error) {
	var observed sessionMetaSnapshot
	if info, err := sessionsRoot.Lstat(path.Join(name, "meta.json")); err == nil && info.Mode().IsRegular() {
		observed = sessionMetaSnapshot{
			identity:       statFileIdentity(info),
			size:           info.Size(),
			modifiedUnixNS: info.ModTime().UnixNano(),
		}
		if observed.identity != "" && previous.payload != nil && observed.sameFile(previous) {
			item, decodeErr := decodeSessionMeta(previous.payload)
			return item, previous, decodeErr == nil, decodeErr
		}
	}
	sessionRoot, err := sessionsRoot.OpenRootNoSymlink(name)
	if err != nil {
		return protocol.Session{}, sessionMetaSnapshot{}, false, nil
	}
	payload, err := sessionRoot.ReadFile("meta.json")
	sessionRoot.Close()
	if errors.Is(err, os.ErrNotExist) {
		return protocol.Session{}, sessionMetaSnapshot{}, false, nil
	}
	if err != nil {
		return protocol.Session{}, sessionMetaSnapshot{}, false, err
	}
	item, err := decodeSessionMeta(payload)
	if err != nil {
		return protocol.Session{}, sessionMetaSnapshot{}, false, err
	}
	// lstat 早于读取：读到的若是之后替换的新文件，下次 lstat 必然与记录不符而重读。
	if observed.identity != "" {
		observed.payload = payload
	}
	return item, observed, true, nil
}

func (s sessionMetaSnapshot) sameFile(other sessionMetaSnapshot) bool {
	return s.identity == other.identity && s.size == other.size && s.modifiedUnixNS == other.modifiedUnixNS
}
