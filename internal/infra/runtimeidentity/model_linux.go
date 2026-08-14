//go:build linux

// INPUT: root-owned launcher 配置、产品 owner 与共享项目授权。
// OUTPUT: 稳定 UID/GID registry、短期启动票据和可执行 policy。
// POS: Linux runtime launcher 的持久身份事实源。
package runtimeidentity

import "time"

const (
	registryVersion = 1

	// v11 一次性归一 Agent workspace 内由旧 nxs 以 0700/0600 创建的
	// AutoMemory 路径；v12 同样归一历史 session summary artifact；v13 在
	// transcript 创建权限统一后修复旧 session artifact。后续托管 runtime
	// 会在创建时保留宿主 ACL mask。
	userLayoutVersion = 13

	defaultTicketTTL   = 24 * time.Hour
	defaultUIDMinimum  = 20000
	defaultUIDMaximum  = 59999
	defaultConfigPath  = "/etc/nexus/runtime-isolation.json"
	launcherVersion    = "2.0.1"
	projectAccessNone  = "none"
	projectAccessRead  = "read"
	projectAccessWrite = "write"
)

type launcherConfig struct {
	StateRoot          string            `json:"state_root"`
	HostUID            int               `json:"host_uid"`
	HostGID            int               `json:"host_gid"`
	UIDMinimum         int               `json:"uid_min"`
	UIDMaximum         int               `json:"uid_max"`
	TicketTTLSeconds   int               `json:"ticket_ttl_seconds"`
	LandlockRequired   bool              `json:"landlock_required"`
	CgroupRoot         string            `json:"cgroup_root,omitempty"`
	CgroupRequired     bool              `json:"cgroup_required,omitempty"`
	RuntimeExecutables map[string]string `json:"runtime_executables"`
	ReadOnlyRoots      []string          `json:"read_only_roots"`
}

type registry struct {
	Version    int                  `json:"version"`
	Generation uint64               `json:"generation"`
	NextID     int                  `json:"next_id"`
	Identities map[string]*identity `json:"identities"`
	Projects   map[string]*project  `json:"projects"`
}

type identity struct {
	OwnerUserID   string `json:"owner_user_id"`
	Username      string `json:"username"`
	GroupName     string `json:"group_name"`
	UID           int    `json:"uid"`
	PrivateGID    int    `json:"private_gid"`
	HomeDir       string `json:"home_dir"`
	TempDir       string `json:"temp_dir"`
	Status        string `json:"status"`
	Generation    uint64 `json:"generation"`
	LayoutVersion int    `json:"layout_version"`
}

type project struct {
	ProjectID  string            `json:"project_id"`
	GroupName  string            `json:"group_name"`
	GID        int               `json:"gid"`
	Root       string            `json:"root"`
	Members    map[string]string `json:"members"`
	Generation uint64            `json:"generation"`
}

type launchTicket struct {
	TicketID           string    `json:"ticket_id"`
	OwnerUserID        string    `json:"owner_user_id"`
	RuntimeKind        string    `json:"runtime_kind"`
	CWD                string    `json:"cwd"`
	RequestedReadRoots []string  `json:"requested_read_roots"`
	EnvironmentNames   []string  `json:"environment_names"`
	Generation         uint64    `json:"generation"`
	CreatedAt          time.Time `json:"created_at"`
	ExpiresAt          time.Time `json:"expires_at"`
}

type preparedPolicy struct {
	OwnerUserID       string           `json:"owner_user_id"`
	RuntimeKind       string           `json:"runtime_kind"`
	CWD               string           `json:"cwd"`
	ReadRoots         []string         `json:"read_roots"`
	WriteRoots        []string         `json:"write_roots"`
	RuntimeReadRoots  []string         `json:"-"`
	RuntimeWriteRoots []string         `json:"-"`
	EnvironmentNames  []string         `json:"-"`
	Generation        uint64           `json:"generation"`
	Ticket            string           `json:"ticket"`
	Identity          preparedIdentity `json:"identity"`
}

type preparedIdentity struct {
	Username          string `json:"username"`
	UID               int    `json:"uid"`
	PrivateGID        int    `json:"private_gid"`
	SupplementaryGIDs []int  `json:"supplementary_gids"`
	HomeDir           string `json:"home_dir"`
	TempDir           string `json:"temp_dir"`
}
