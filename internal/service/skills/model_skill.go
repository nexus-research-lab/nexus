// INPUT: Skill frontmatter、来源元数据与目标 Agent 的目录状态。
// OUTPUT: 共享系统 Skill 目录、列表/详情模型及绑定 runtime_version 的 AgentSkillState。
// POS: Skills 服务跨目录、HTTP、配置控制面与 runtimecommand 绑定的协议模型。
package skills

import (
	"context"
	_ "embed"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/runtimecommand"
)

const (
	sourceTypeSystem        = "system"
	sourceTypeBuiltin       = "builtin"
	sourceTypeExternal      = "external"
	sourceTypeWorkspace     = "workspace"
	sourceKindNexusPlatform = "nexus_platform"
	sourceKindUserGlobal    = "user_global"
	storageScopePlatform    = "platform"
	storageScopeUserGlobal  = "user_global"
	storageScopeAgent       = "agent_workspace"
	originKindBuiltin       = "builtin"
	originKindUserImport    = "user_import"
	originKindMarketplace   = "marketplace"
	originKindAgentCreated  = "agent_created"
	scopeMain               = "main"
	scopeAny                = "any"
	scopeRoom               = "room"
)

// ScopeRoom 表示 Room 级 skill，只能由房间启用，不能绑定到单个 Agent。
const ScopeRoom = scopeRoom

// AgentSkillTargetScope 表示 Agent 开关要操作的 Skill 来源作用域。
type AgentSkillTargetScope string

const (
	// AgentSkillTargetGlobalLibrary 表示用户全局技能库中的 Skill。
	AgentSkillTargetGlobalLibrary AgentSkillTargetScope = "global_library"
	// AgentSkillTargetWorkspace 表示当前 Agent workspace 的私有 Skill。
	AgentSkillTargetWorkspace AgentSkillTargetScope = "agent_workspace"
)

var (
	systemSkillNames   = buildSystemSkillNames()
	internalSkillNames = map[string]struct{}{
		"nexus-manager": {},
	}
	curatedEntriesOnce sync.Once
	curatedEntriesData map[string]map[string]string
	curatedEntriesErr  error
)

func buildSystemSkillNames() map[string]struct{} {
	result := map[string]struct{}{
		"imagegen":            {},
		"visualize":           {},
		"automation":          {},
		"nexus-configuration": {},
	}
	for _, skillName := range runtimecommand.ManagedSemanticSkillNames() {
		result[skillName] = struct{}{}
	}
	return result
}

// 中文注释：catalog 元数据直接编进二进制，避免运行时容器再依赖源码目录。
//
//go:embed data/curated_skill_catalog.json
var curatedCatalogPayload []byte

// Info 表示 skill 列表项。
type Info struct {
	Name              string                `json:"name"`
	Title             string                `json:"title"`
	Description       string                `json:"description"`
	Scope             string                `json:"scope"`
	Tags              []string              `json:"tags"`
	CategoryKey       string                `json:"category_key"`
	CategoryName      string                `json:"category_name"`
	SourceType        string                `json:"source_type"`
	SourceRef         string                `json:"source_ref"`
	Version           string                `json:"version"`
	Locked            bool                  `json:"locked"`
	HasUpdate         bool                  `json:"has_update"`
	Deletable         bool                  `json:"deletable"`
	SourceKind        string                `json:"source_kind,omitempty"`
	SourceName        string                `json:"source_name,omitempty"`
	SourceTrust       string                `json:"source_trust,omitempty"`
	ImportMode        string                `json:"import_mode,omitempty"`
	LastError         string                `json:"last_error,omitempty"`
	StorageScope      string                `json:"storage_scope,omitempty"`
	OriginKind        string                `json:"origin_kind,omitempty"`
	TargetScope       AgentSkillTargetScope `json:"target_scope,omitempty"`
	SourceIdentity    string                `json:"source_identity,omitempty"`
	EnabledForAgent   bool                  `json:"enabled_for_agent"`
	EnabledAgentCount int                   `json:"enabled_agent_count,omitempty"`
}

// Detail 表示 skill 详情。
type Detail struct {
	Info
	ReadmeMarkdown string `json:"readme_markdown"`
	Recommendation string `json:"recommendation"`
	// 兼容既有前端字段名；当前表示用户级源同步结果，而非 workspace 复制结果。
	DeploySuccesses []RedeployAgentSuccess `json:"deploy_successes,omitempty"`
	DeployFailures  []RedeployAgentFailure `json:"deploy_failures,omitempty"`
}

// AgentSkillBinding 表示某个 Skill 在单个 Agent 上的启用投影。
type AgentSkillBinding struct {
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	IsMain    bool   `json:"is_main"`
	Available bool   `json:"available"`
	Enabled   bool   `json:"enabled"`
}

// Query 表示技能查询参数。
type Query struct {
	AgentID     string
	CategoryKey string
	SourceType  string
	Scope       string
	Q           string
}

// AgentSkillState 是一次目录读取中绑定的 Agent 版本与目标 Skill 安装状态。
type AgentSkillState struct {
	AgentID        string                `json:"agent_id"`
	RuntimeVersion int64                 `json:"runtime_version"`
	SkillName      string                `json:"skill_name"`
	TargetScope    AgentSkillTargetScope `json:"target_scope,omitempty"`
	SourceIdentity string                `json:"source_identity,omitempty"`
	Available      bool                  `json:"available"`
	Installed      bool                  `json:"installed"`
	Locked         bool                  `json:"locked,omitempty"`
	Scope          string                `json:"scope,omitempty"`
	SourceType     string                `json:"source_type,omitempty"`
	SourceKind     string                `json:"source_kind,omitempty"`
	SourceRef      string                `json:"source_ref,omitempty"`
	StorageScope   string                `json:"storage_scope,omitempty"`
	OriginKind     string                `json:"origin_kind,omitempty"`
	Version        string                `json:"version,omitempty"`
}

type curatedCatalog struct {
	Skills []struct {
		Name           string `json:"name"`
		CategoryKey    string `json:"category_key"`
		CategoryName   string `json:"category_name"`
		Recommendation string `json:"recommendation"`
	} `json:"skills"`
}

type externalManifest struct {
	Name           string   `json:"name"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Scope          string   `json:"scope"`
	Tags           []string `json:"tags"`
	CategoryKey    string   `json:"category_key"`
	CategoryName   string   `json:"category_name"`
	Version        string   `json:"version"`
	SourceType     string   `json:"source_type"`
	SourceRef      string   `json:"source_ref"`
	SourceKind     string   `json:"source_kind"`
	SourceKey      string   `json:"source_key"`
	SourceName     string   `json:"source_name"`
	SourceTrust    string   `json:"source_trust"`
	SourceSkillID  string   `json:"source_skill_id"`
	ArtifactSHA256 string   `json:"artifact_sha256"`
	ImportMode     string   `json:"import_mode"`
	Recommendation string   `json:"recommendation"`
	GitURL         string   `json:"git_url"`
	GitBranch      string   `json:"git_branch"`
	GitPath        string   `json:"git_path"`
	GitCommit      string   `json:"git_commit"`
	RawURL         string   `json:"raw_url"`
	DetailURL      string   `json:"detail_url"`
}

type catalogRecord struct {
	Detail     Detail
	SourcePath string
	Manifest   externalManifest
}

type commandRunnerFunc func(ctx context.Context, workDir string, extraEnv []string, command ...string) (string, error)
