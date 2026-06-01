package skills

import (
	"net/http"
	"regexp"
	"time"
)

const (
	maxExternalPreviewBytes       = 2 * 1024 * 1024
	maxExternalImportBytes        = 32 * 1024 * 1024
	maxExternalPreviewConcurrency = 4
	gitCloneMaxAttempts           = 3

	externalSourceKindClaudePlugins = "claude_plugins"
	externalSourceKindSkillsSh      = "skills_sh"
	externalSourceKindClawhub       = "clawhub"
	externalSourceKindHermesIndex   = "hermes_index"
	externalSourceKindBrowseSh      = "browse_sh"
	externalSourceKindWellKnown     = "well_known"
	externalSourceKindGit           = "git"
	externalSourceKindURL           = "url"
	externalSourceKindUploaded      = "uploaded_zip"
	externalSourceKindLocalPath     = "local_path"

	externalSourceTrustOfficial  = "official"
	externalSourceTrustCommunity = "community"
	externalSourceTrustPrivate   = "private"

	defaultClaudePluginsSearchURL = "https://claude-plugins.dev/api/skills"
	defaultSkillsShURL            = "https://skills.sh"
	defaultClawhubSearchURL       = "https://clawhub.ai/api/v1/search"
	defaultHermesIndexURL         = "https://hermes-agent.nousresearch.com/docs/api/skills-index.json"
	defaultBrowseShURL            = "https://browse.sh/api/skills"
)

type gitCloneOptions struct {
	Branch string
	// CleanGlobalConfig 用于公开 marketplace 来源，避免本机 url.insteadOf 把 HTTPS 重写成 SSH。
	CleanGlobalConfig bool
}

var (
	externalSkillsHTTPClient = &http.Client{Timeout: 20 * time.Second}
	previewMarkdownRules     = []struct {
		pattern *regexp.Regexp
		replace string
	}{
		{regexp.MustCompile(`<pre.*?><code.*?>`), "```text\n"},
		{regexp.MustCompile(`</code></pre>`), "\n```"},
		{regexp.MustCompile(`<h1>(.*?)</h1>`), "# $1\n"},
		{regexp.MustCompile(`<h2>(.*?)</h2>`), "## $1\n"},
		{regexp.MustCompile(`<h3>(.*?)</h3>`), "### $1\n"},
		{regexp.MustCompile(`<li>(.*?)</li>`), "- $1"},
		{regexp.MustCompile(`<p>(.*?)</p>`), "$1\n"},
		{regexp.MustCompile(`<[^>]+>`), ""},
	}
)

// ExternalSkillSearchItem 表示外部技能搜索结果。
type ExternalSkillSearchItem struct {
	Name           string   `json:"name"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Source         string   `json:"source"`
	PackageSpec    string   `json:"package_spec"`
	SkillSlug      string   `json:"skill_slug"`
	Installs       int      `json:"installs"`
	DetailURL      string   `json:"detail_url"`
	ReadmeMarkdown string   `json:"readme_markdown"`
	SourceKind     string   `json:"source_kind"`
	SourceKey      string   `json:"source_key"`
	SourceName     string   `json:"source_name"`
	SourceTrust    string   `json:"source_trust"`
	ImportMode     string   `json:"import_mode"`
	GitURL         string   `json:"git_url"`
	GitBranch      string   `json:"git_branch"`
	GitPath        string   `json:"git_path"`
	RawURL         string   `json:"raw_url"`
	Tags           []string `json:"tags"`
	Version        string   `json:"version"`
}

// ExternalSkillSourceStatus 表示一次外部来源搜索状态。
type ExternalSkillSourceStatus struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ExternalSkillSourceInfo 表示可管理的外部 skill 来源配置。
type ExternalSkillSourceInfo struct {
	SourceID      string     `json:"source_id"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"`
	URL           string     `json:"url"`
	Trust         string     `json:"trust"`
	Enabled       bool       `json:"enabled"`
	SortOrder     int        `json:"sort_order"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}

// ExternalSkillSourceRequest 表示外部 skill 来源开关请求。
type ExternalSkillSourceRequest struct {
	Enabled *bool `json:"enabled"`
}

// SearchExternalSkillsResponse 表示外部技能搜索响应。
type SearchExternalSkillsResponse struct {
	Query   string                      `json:"query"`
	Results []ExternalSkillSearchItem   `json:"results"`
	Sources []ExternalSkillSourceStatus `json:"sources"`
}

// ExternalSkillPreviewResponse 表示技能详情预览。
type ExternalSkillPreviewResponse struct {
	DetailURL      string `json:"detail_url"`
	ReadmeMarkdown string `json:"readme_markdown"`
}

type externalSkillSource struct {
	Key       string
	Name      string
	Kind      string
	URL       string
	Trust     string
	Enabled   bool
	SortOrder int
}

// SkillActionFailure 表示单个技能动作失败结果。
type SkillActionFailure struct {
	SkillName string `json:"skill_name"`
	Error     string `json:"error"`
}

// UpdateInstalledSkillsResponse 表示批量更新结果。
type UpdateInstalledSkillsResponse struct {
	UpdatedSkills []string             `json:"updated_skills"`
	SkippedSkills []string             `json:"skipped_skills"`
	Failures      []SkillActionFailure `json:"failures"`
}
