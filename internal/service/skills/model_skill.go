package skills

import (
	"context"
	_ "embed"
	"sync"
)

const (
	sourceTypeSystem    = "system"
	sourceTypeBuiltin   = "builtin"
	sourceTypeExternal  = "external"
	sourceTypeWorkspace = "workspace"
	scopeMain           = "main"
	scopeAny            = "any"
	scopeRoom           = "room"

	registryUsersDirName            = "users"
	registryLegacyMigratedDirName   = "legacy-migrated"
	registryLegacyUnassignedDirName = "legacy-unassigned"
)

// ScopeRoom 表示 Room 级 skill，只能由房间启用，不能安装到单个 Agent。
const ScopeRoom = scopeRoom

var (
	systemSkillNames   = map[string]struct{}{"imagegen": {}, "memory-manager": {}, "scheduled-task-manager": {}, "goal-manager": {}}
	internalSkillNames = map[string]struct{}{"nexus-manager": {}}
	curatedEntriesOnce sync.Once
	curatedEntriesData map[string]map[string]string
	curatedEntriesErr  error
)

// 中文注释：catalog 元数据直接编进二进制，避免运行时容器再依赖源码目录。
//
//go:embed data/curated_skill_catalog.json
var curatedCatalogPayload []byte

// Info 表示 skill 列表项。
type Info struct {
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Scope        string   `json:"scope"`
	Tags         []string `json:"tags"`
	CategoryKey  string   `json:"category_key"`
	CategoryName string   `json:"category_name"`
	SourceType   string   `json:"source_type"`
	SourceRef    string   `json:"source_ref"`
	Version      string   `json:"version"`
	Installed    bool     `json:"installed"`
	Locked       bool     `json:"locked"`
	HasUpdate    bool     `json:"has_update"`
	Deletable    bool     `json:"deletable"`
	SourceKind   string   `json:"source_kind,omitempty"`
	SourceName   string   `json:"source_name,omitempty"`
	SourceTrust  string   `json:"source_trust,omitempty"`
	ImportMode   string   `json:"import_mode,omitempty"`
	LastError    string   `json:"last_error,omitempty"`
}

// Detail 表示 skill 详情。
type Detail struct {
	Info
	ReadmeMarkdown string `json:"readme_markdown"`
	Recommendation string `json:"recommendation"`
}

// Query 表示技能查询参数。
type Query struct {
	AgentID     string
	CategoryKey string
	SourceType  string
	Scope       string
	Q           string
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
}

type commandRunnerFunc func(ctx context.Context, workDir string, extraEnv []string, command ...string) (string, error)
