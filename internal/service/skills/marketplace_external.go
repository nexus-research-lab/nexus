package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

const (
	maxExternalPreviewBytes       = 2 * 1024 * 1024
	maxExternalImportBytes        = 32 * 1024 * 1024
	maxExternalPreviewConcurrency = 4

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

// ImportUploadedArchive 从浏览器上传的 zip 导入技能。
func (s *Service) ImportUploadedArchive(ctx context.Context, filename string, payload []byte) (*Detail, error) {
	if len(payload) == 0 {
		return nil, errors.New("上传文件为空")
	}
	tempDir, err := os.MkdirTemp("", "nexus-skill-upload-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	if err = unzipArchive(payload, tempDir); err != nil {
		return nil, err
	}
	sourceDir, err := findSkillSourceDir(tempDir)
	if err != nil {
		return nil, err
	}
	return s.importSourceDir(ctx, sourceDir, externalManifest{
		SourceType:     sourceTypeExternal,
		SourceRef:      strings.TrimSpace(filename),
		SourceKind:     externalSourceKindUploaded,
		SourceName:     "本地上传",
		SourceTrust:    externalSourceTrustPrivate,
		ImportMode:     externalSourceKindUploaded,
		Version:        "uploaded",
		Recommendation: "来自本地上传。",
	})
}

// ImportGit 从 Git 仓库导入技能。
func (s *Service) ImportGit(ctx context.Context, repositoryURL string, branch string) (*Detail, error) {
	return s.importGit(ctx, repositoryURL, branch, "", externalManifest{})
}

// ImportGitPath 从 Git 仓库的指定子目录导入技能。
func (s *Service) ImportGitPath(ctx context.Context, repositoryURL string, branch string, skillPath string) (*Detail, error) {
	return s.importGit(ctx, repositoryURL, branch, skillPath, externalManifest{})
}

func (s *Service) importGit(ctx context.Context, repositoryURL string, branch string, skillPath string, manifest externalManifest) (*Detail, error) {
	repositoryURL = strings.TrimSpace(repositoryURL)
	if repositoryURL == "" {
		return nil, errors.New("url 不能为空")
	}
	if parsed, parseErr := url.Parse(repositoryURL); parseErr != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return nil, errors.New("仅支持 https:// 协议的 Git 仓库地址")
	}
	cleanSkillPath, err := cleanSkillSubdirPath(skillPath)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "nexus-skill-git-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	command := []string{"git", "clone", "--depth", "1"}
	if strings.TrimSpace(branch) != "" {
		command = append(command, "--branch", strings.TrimSpace(branch))
	}
	command = append(command, repositoryURL, tempDir)
	if output, runErr := s.runCommand(ctx, "", command...); runErr != nil {
		return nil, fmt.Errorf("Git 导入失败: %s", output)
	}
	sourceRoot := tempDir
	if cleanSkillPath != "" {
		sourceRoot = filepath.Join(tempDir, cleanSkillPath)
	}
	sourceDir, err := findSkillSourceDir(sourceRoot)
	if err != nil {
		return nil, err
	}
	commitOutput, revErr := s.runCommand(ctx, tempDir, "git", "rev-parse", "HEAD")
	if revErr != nil {
		slog.WarnContext(ctx, "git rev-parse HEAD 失败", "repository_url", repositoryURL, "err", revErr)
	}
	manifest.SourceType = sourceTypeExternal
	manifest.SourceRef = firstNonEmpty(manifest.SourceRef, repositoryURL)
	manifest.SourceKind = firstNonEmpty(manifest.SourceKind, externalSourceKindGit)
	manifest.SourceKey = firstNonEmpty(manifest.SourceKey, repositoryURL)
	manifest.SourceName = firstNonEmpty(manifest.SourceName, "Git")
	manifest.SourceTrust = firstNonEmpty(manifest.SourceTrust, externalSourceTrustCommunity)
	manifest.ImportMode = "git"
	manifest.GitURL = repositoryURL
	manifest.GitBranch = strings.TrimSpace(branch)
	manifest.GitPath = filepath.ToSlash(cleanSkillPath)
	manifest.GitCommit = strings.TrimSpace(commitOutput)
	manifest.Version = firstNonEmpty(strings.TrimSpace(commitOutput), manifest.Version, "git")
	return s.importSourceDir(ctx, sourceDir, manifest)
}

// SearchExternalSkills 聚合搜索配置化的外部技能来源。
func (s *Service) SearchExternalSkills(ctx context.Context, query string, includeReadme bool) (*SearchExternalSkillsResponse, error) {
	needle := strings.TrimSpace(query)
	if needle == "" {
		return &SearchExternalSkillsResponse{Query: "", Results: []ExternalSkillSearchItem{}, Sources: []ExternalSkillSourceStatus{}}, nil
	}
	sources := s.externalSkillSources(ctx)
	if len(sources) == 0 {
		return nil, errors.New("未配置可搜索的 skill 来源")
	}
	type searchResult struct {
		index  int
		source externalSkillSource
		items  []ExternalSkillSearchItem
		err    error
	}
	resultCh := make(chan searchResult, len(sources))
	for index, source := range sources {
		index := index
		source := source
		go func() {
			sourceItems, err := s.searchExternalSkillSource(ctx, source, needle)
			resultCh <- searchResult{
				index:  index,
				source: source,
				items:  sourceItems,
				err:    err,
			}
		}()
	}
	items := make([]ExternalSkillSearchItem, 0)
	statuses := make([]ExternalSkillSourceStatus, len(sources))
	failedSources := 0
	for range sources {
		result := <-resultCh
		source := result.source
		status := ExternalSkillSourceStatus{
			Key:    source.Key,
			Name:   source.Name,
			Kind:   source.Kind,
			URL:    source.URL,
			Status: "ok",
		}
		if result.err != nil {
			failedSources++
			status.Status = "error"
			status.Error = result.err.Error()
			s.recordExternalSourceCheck(ctx, source, result.err.Error())
			slog.WarnContext(ctx, "外部 skill 来源搜索失败", "source", source.Name, "kind", source.Kind, "err", result.err)
			statuses[result.index] = status
			continue
		}
		s.recordExternalSourceCheck(ctx, source, "")
		items = append(items, result.items...)
		statuses[result.index] = status
	}
	if failedSources == len(sources) {
		return nil, errors.New("所有外部 skill 来源搜索失败")
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Installs != items[j].Installs {
			return items[i].Installs > items[j].Installs
		}
		if items[i].SourceName != items[j].SourceName {
			return items[i].SourceName < items[j].SourceName
		}
		return items[i].Name < items[j].Name
	})
	items = dedupeExternalItems(items)
	if includeReadme {
		s.attachExternalReadmes(ctx, items)
	}
	return &SearchExternalSkillsResponse{Query: needle, Results: items, Sources: statuses}, nil
}

func (s *Service) recordExternalSourceCheck(ctx context.Context, source externalSkillSource, lastError string) {
	if s.skillStore == nil || strings.TrimSpace(source.Key) == "" {
		return
	}
	if err := s.skillStore.RecordSourceCheck(ctx, authctx.OwnerUserID(ctx), source.Key, time.Now().UTC(), lastError); err != nil {
		slog.WarnContext(ctx, "记录 skill 来源检查状态失败", "source", source.Name, "err", err)
	}
}

// GetExternalSkillPreview 获取外部技能详情页或 README 的预览。
func (s *Service) GetExternalSkillPreview(ctx context.Context, detailURL string) (*ExternalSkillPreviewResponse, error) {
	targetURL, err := s.validateExternalURL(ctx, detailURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills 预览加载失败: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalPreviewBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxExternalPreviewBytes {
		body = body[:maxExternalPreviewBytes]
	}
	return &ExternalSkillPreviewResponse{
		DetailURL:      targetURL,
		ReadmeMarkdown: extractPreviewMarkdown(string(body)),
	}, nil
}

func (s *Service) attachExternalReadmes(ctx context.Context, items []ExternalSkillSearchItem) {
	if len(items) == 0 {
		return
	}
	semaphore := make(chan struct{}, maxExternalPreviewConcurrency)
	var wg sync.WaitGroup
	for index := range items {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			preview, err := s.GetExternalSkillPreview(ctx, items[index].DetailURL)
			if err == nil && preview != nil {
				items[index].ReadmeMarkdown = preview.ReadmeMarkdown
			}
		}()
	}
	wg.Wait()
}

// ImportSkillsSh 从 skills.sh 搜索结果导入技能。
func (s *Service) ImportSkillsSh(ctx context.Context, packageSpec string, skillSlug string) (*Detail, error) {
	packageSpec = strings.TrimSpace(packageSpec)
	skillSlug = strings.TrimSpace(skillSlug)
	if packageSpec == "" {
		return nil, errors.New("package_spec 不能为空")
	}
	if skillSlug == "" {
		return nil, errors.New("skill_slug 不能为空")
	}
	tempDir, err := os.MkdirTemp("", "nexus-skills-sh-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	packageJSONPath := filepath.Join(tempDir, "package.json")
	if err = os.WriteFile(packageJSONPath, []byte("{\"private\":true}\n"), 0o644); err != nil {
		return nil, err
	}
	if strings.Contains(packageSpec, "@") {
		if output, runErr := s.runPnpmCommand(ctx, tempDir, "dlx", "skills", "add", packageSpec, "-y", "--copy"); runErr != nil {
			return nil, fmt.Errorf("skills.sh 导入失败: %s", output)
		}
	} else {
		if output, runErr := s.runPnpmCommand(ctx, tempDir, "dlx", "skills", "add", packageSpec, "--skill", skillSlug, "-y", "--copy"); runErr != nil {
			return nil, fmt.Errorf("skills.sh 导入失败: %s", output)
		}
	}
	sourceDir, err := findSkillSourceDir(tempDir)
	if err != nil {
		return nil, err
	}
	return s.importSourceDir(ctx, sourceDir, externalManifest{
		SourceType:  sourceTypeExternal,
		SourceRef:   packageSpec,
		SourceKind:  externalSourceKindSkillsSh,
		SourceKey:   firstNonEmpty(strings.TrimSpace(s.config.SkillsAPIURL), "https://skills.sh"),
		SourceName:  "skills.sh",
		SourceTrust: externalSourceTrustCommunity,
		ImportMode:  "skills_sh",
		Version:     packageSpec,
	})
}

// ImportExternalSkill 按搜索结果携带的来源信息导入技能。
func (s *Service) ImportExternalSkill(ctx context.Context, item ExternalSkillSearchItem) (*Detail, error) {
	mode := normalizeImportMode(firstNonEmpty(item.ImportMode, inferExternalImportMode(item)))
	manifest := externalManifest{
		Name:           strings.TrimSpace(item.SkillSlug),
		Title:          strings.TrimSpace(item.Title),
		Description:    strings.TrimSpace(item.Description),
		Tags:           normalizeStringSlice(item.Tags),
		Version:        strings.TrimSpace(item.Version),
		SourceType:     sourceTypeExternal,
		SourceRef:      firstNonEmpty(item.PackageSpec, item.RawURL, item.GitURL, item.DetailURL),
		SourceKind:     strings.TrimSpace(item.SourceKind),
		SourceKey:      strings.TrimSpace(item.SourceKey),
		SourceName:     strings.TrimSpace(item.SourceName),
		SourceTrust:    strings.TrimSpace(item.SourceTrust),
		ImportMode:     mode,
		Recommendation: firstNonEmpty(item.Description, "外部导入能力。"),
		GitURL:         strings.TrimSpace(item.GitURL),
		GitBranch:      strings.TrimSpace(item.GitBranch),
		GitPath:        strings.TrimSpace(item.GitPath),
		RawURL:         strings.TrimSpace(item.RawURL),
		DetailURL:      strings.TrimSpace(item.DetailURL),
	}
	switch mode {
	case externalSourceKindSkillsSh:
		return s.ImportSkillsSh(ctx, item.PackageSpec, item.SkillSlug)
	case externalSourceKindGit:
		repositoryURL := firstNonEmpty(item.GitURL, item.PackageSpec, item.Source)
		return s.importGit(ctx, repositoryURL, item.GitBranch, item.GitPath, manifest)
	case externalSourceKindURL:
		sourceURL := firstNonEmpty(item.RawURL, item.DetailURL, item.PackageSpec, item.Source)
		return s.ImportSkillURL(ctx, sourceURL, manifest)
	default:
		return nil, errors.New("不支持的外部 skill 来源")
	}
}

// ImportSkillURL 从可信外部 URL 导入 SKILL.md 或 zip 归档。
func (s *Service) ImportSkillURL(ctx context.Context, sourceURL string, manifest externalManifest) (*Detail, error) {
	targetURL, err := s.validateExternalURL(ctx, sourceURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skill URL 导入失败: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalImportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxExternalImportBytes {
		return nil, errors.New("skill URL 内容超过大小限制")
	}
	tempDir, err := os.MkdirTemp("", "nexus-skill-url-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	if strings.HasSuffix(strings.ToLower(targetURL), ".zip") {
		if err = unzipArchive(body, tempDir); err != nil {
			return nil, err
		}
	} else {
		if err = os.WriteFile(filepath.Join(tempDir, "SKILL.md"), body, 0o644); err != nil {
			return nil, err
		}
	}
	sourceDir, err := findSkillSourceDir(tempDir)
	if err != nil {
		return nil, err
	}
	manifest.SourceType = sourceTypeExternal
	manifest.SourceRef = firstNonEmpty(manifest.SourceRef, targetURL)
	manifest.SourceKind = firstNonEmpty(manifest.SourceKind, externalSourceKindURL)
	manifest.SourceKey = firstNonEmpty(manifest.SourceKey, targetURL)
	manifest.SourceName = firstNonEmpty(manifest.SourceName, "URL")
	manifest.SourceTrust = firstNonEmpty(manifest.SourceTrust, externalSourceTrustCommunity)
	manifest.ImportMode = externalSourceKindURL
	manifest.RawURL = targetURL
	manifest.Version = firstNonEmpty(manifest.Version, targetURL)
	return s.importSourceDir(ctx, sourceDir, manifest)
}

// ListExternalSkillSources 返回当前用户的社区 skill 来源配置。
func (s *Service) ListExternalSkillSources(ctx context.Context) ([]ExternalSkillSourceInfo, error) {
	configuredSources := s.configuredExternalSkillSources()
	if s.skillStore == nil {
		items := make([]ExternalSkillSourceInfo, 0, len(configuredSources))
		for _, source := range configuredSources {
			items = append(items, externalSkillSourceInfoFromSource(source))
		}
		return items, nil
	}
	if err := s.ensureConfiguredSkillSources(ctx, configuredSources); err != nil {
		return nil, err
	}
	rows, err := s.skillStore.ListSources(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	configuredIDs := configuredExternalSourceIDs(configuredSources)
	items := make([]ExternalSkillSourceInfo, 0, len(rows))
	for _, row := range rows {
		if _, ok := configuredIDs[row.SourceID]; !ok {
			continue
		}
		items = append(items, externalSkillSourceInfoFromEntity(row))
	}
	return items, nil
}

// UpdateExternalSkillSource 更新当前用户的社区 skill 来源开关。
func (s *Service) UpdateExternalSkillSource(ctx context.Context, sourceID string, request ExternalSkillSourceRequest) (*ExternalSkillSourceInfo, error) {
	sourceID = strings.TrimSpace(sourceID)
	if s.skillStore == nil {
		return nil, errors.New("skill source store not configured")
	}
	configuredSources := s.configuredExternalSkillSources()
	if err := s.ensureConfiguredSkillSources(ctx, configuredSources); err != nil {
		return nil, err
	}
	if _, ok := configuredExternalSourceIDs(configuredSources)[sourceID]; !ok {
		return nil, errors.New("skill source not found")
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	existing, err := s.skillStore.GetSource(ctx, ownerUserID, sourceID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("skill source not found")
	}
	enabled := existing.Enabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	entity := *existing
	entity.Enabled = enabled
	if err = s.skillStore.UpsertSource(ctx, entity); err != nil {
		return nil, err
	}
	row, err := s.skillStore.GetSource(ctx, ownerUserID, existing.SourceID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.New("skill source not found")
	}
	item := externalSkillSourceInfoFromEntity(*row)
	return &item, nil
}

// UpdateImportedSkills 更新所有已导入的外部技能。
func (s *Service) UpdateImportedSkills(ctx context.Context) (*UpdateInstalledSkillsResponse, error) {
	records, err := s.loadExternalRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := &UpdateInstalledSkillsResponse{
		UpdatedSkills: make([]string, 0),
		SkippedSkills: make([]string, 0),
		Failures:      make([]SkillActionFailure, 0),
	}
	names := make([]string, 0, len(records))
	for name := range records {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, updateErr := s.updateSingleSkillRecord(ctx, records[name]); updateErr != nil {
			if strings.Contains(updateErr.Error(), "不支持更新") {
				result.SkippedSkills = append(result.SkippedSkills, name)
				continue
			}
			result.Failures = append(result.Failures, SkillActionFailure{
				SkillName: name,
				Error:     updateErr.Error(),
			})
			continue
		}
		result.UpdatedSkills = append(result.UpdatedSkills, name)
	}
	return result, nil
}

// UpdateSingleSkill 更新单个已导入技能。
func (s *Service) UpdateSingleSkill(ctx context.Context, skillName string) (*Detail, error) {
	records, err := s.loadExternalRecords(ctx)
	if err != nil {
		return nil, err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return nil, errors.New("skill not found")
	}
	return s.updateSingleSkillRecord(ctx, record)
}

func (s *Service) updateSingleSkillRecord(ctx context.Context, record catalogRecord) (*Detail, error) {
	manifest, err := s.readManifest(record.SourcePath)
	if err != nil {
		return nil, err
	}
	switch manifest.ImportMode {
	case "git":
		return s.importGit(ctx, manifest.GitURL, manifest.GitBranch, manifest.GitPath, manifest)
	case "skills_sh":
		return s.ImportSkillsSh(ctx, manifest.SourceRef, manifest.Name)
	case "url":
		return s.ImportSkillURL(ctx, firstNonEmpty(manifest.RawURL, manifest.SourceRef, manifest.DetailURL), manifest)
	default:
		return nil, errors.New("该 skill 来源不支持更新")
	}
}

func (s *Service) importSourceDir(ctx context.Context, sourceDir string, manifest externalManifest) (*Detail, error) {
	content, skillMDPath, skillName, err := readSkillSource(sourceDir)
	if err != nil {
		return nil, err
	}
	parsed := parseSkillFrontmatter(content, skillName)
	if parsed.Name == "" {
		return nil, errors.New("SKILL.md 缺少 name")
	}
	targetDir := filepath.Join(s.registryRoot(ctx), parsed.Name)
	if err = os.RemoveAll(targetDir); err != nil {
		return nil, err
	}
	if err = copyDirectory(filepath.Dir(skillMDPath), targetDir); err != nil {
		return nil, err
	}
	manifest.Name = parsed.Name
	manifest.Title = firstNonEmpty(parsed.Title, manifest.Title, parsed.Name)
	manifest.Description = firstNonEmpty(parsed.Description, manifest.Description)
	manifest.Scope = defaultSkillScope(firstNonEmpty(parsed.Scope, manifest.Scope))
	manifest.Tags = firstNonEmptySlice(parsed.Tags, manifest.Tags)
	manifest.CategoryKey = firstNonEmpty(manifest.CategoryKey, parsed.CategoryKey, "custom-imports")
	manifest.CategoryName = firstNonEmpty(manifest.CategoryName, parsed.CategoryName, "自定义导入")
	manifest.Recommendation = firstNonEmpty(manifest.Recommendation, parsed.Recommendation, "外部导入能力。")
	manifest.SourceType = sourceTypeExternal
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(targetDir, ".nexus-skill.json"), payload, 0o644); err != nil {
		return nil, err
	}
	if err = s.upsertImportedSkillRecord(ctx, targetDir, manifest, parsed); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, err
	}
	return s.GetSkillDetail(ctx, parsed.Name, "")
}

func (s *Service) readManifest(skillDir string) (externalManifest, error) {
	payload, err := os.ReadFile(filepath.Join(skillDir, ".nexus-skill.json"))
	if err != nil {
		return externalManifest{}, err
	}
	var manifest externalManifest
	if err = json.Unmarshal(payload, &manifest); err != nil {
		return externalManifest{}, err
	}
	return manifest, nil
}

func (s *Service) runCommand(ctx context.Context, workDir string, command ...string) (string, error) {
	return s.runCommandWithEnv(ctx, workDir, nil, command...)
}

func (s *Service) runCommandWithEnv(ctx context.Context, workDir string, extraEnv []string, command ...string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("命令不能为空")
	}
	if s.commandRunner != nil {
		return s.commandRunner(ctx, workDir, extraEnv, command...)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, extraEnv...)
	if strings.TrimSpace(s.config.SkillsAPIURL) != "" {
		cmd.Env = append(cmd.Env, "SKILLS_API_URL="+strings.TrimSpace(s.config.SkillsAPIURL))
	}
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func (s *Service) runPnpmCommand(ctx context.Context, workDir string, args ...string) (string, error) {
	command := []string{"pnpm"}
	extraEnv := make([]string, 0, 2)
	if registry := strings.TrimSpace(s.config.PnpmRegistry); registry != "" {
		extraEnv = append(extraEnv, "npm_config_registry="+registry)
	}
	if cacheRoot := strings.TrimSpace(s.config.CacheFileDir); cacheRoot != "" {
		storeDir := filepath.Join(cacheRoot, "pnpm-store")
		_ = os.MkdirAll(storeDir, 0o755)
		extraEnv = append(extraEnv, "npm_config_store_dir="+storeDir)
	}
	command = append(command, args...)
	return s.runCommandWithEnv(ctx, workDir, extraEnv, command...)
}

func unzipArchive(payload []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return errors.New("上传文件不是合法 zip 包")
	}
	for _, file := range reader.File {
		targetPath := filepath.Join(targetDir, file.Name)
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return errors.New("zip 包含非法路径")
		}
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(cleanTarget, 0o755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
			return err
		}
		readerHandle, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		writer, createErr := os.OpenFile(cleanTarget, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if createErr != nil {
			_ = readerHandle.Close()
			return createErr
		}
		if _, err = io.Copy(writer, readerHandle); err != nil {
			_ = readerHandle.Close()
			_ = writer.Close()
			return err
		}
		if err = readerHandle.Close(); err != nil {
			_ = writer.Close()
			return err
		}
		if err = writer.Close(); err != nil {
			return err
		}
	}
	return nil
}

func findSkillSourceDir(root string) (string, error) {
	bestMatch := ""
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() != "SKILL.md" {
			return nil
		}
		sourceDir := filepath.Dir(path)
		if bestMatch == "" || len(sourceDir) < len(bestMatch) {
			bestMatch = sourceDir
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if bestMatch == "" {
		return "", errors.New("未找到 SKILL.md")
	}
	return bestMatch, nil
}

func buildSkillsPackageSpec(source string, slug string, name string) string {
	base := firstNonEmpty(strings.TrimSpace(source), strings.TrimSpace(slug))
	if base == "" {
		return name
	}
	if strings.Contains(base, "@") {
		return base
	}
	return base + "@" + name
}

func (s *Service) defaultExternalSkillSources() []externalSkillSource {
	skillsShURL := strings.TrimRight(firstNonEmpty(strings.TrimSpace(s.config.SkillsAPIURL), defaultSkillsShURL), "/")
	return []externalSkillSource{
		{
			Key:       buildSkillSourceID(externalSourceKindClaudePlugins, defaultClaudePluginsSearchURL),
			Name:      "claude-plugins.dev",
			Kind:      externalSourceKindClaudePlugins,
			URL:       defaultClaudePluginsSearchURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 0,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindSkillsSh, skillsShURL),
			Name:      "skills.sh",
			Kind:      externalSourceKindSkillsSh,
			URL:       skillsShURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 10,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindClawhub, defaultClawhubSearchURL),
			Name:      "clawhub.ai",
			Kind:      externalSourceKindClawhub,
			URL:       defaultClawhubSearchURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 20,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindBrowseSh, defaultBrowseShURL),
			Name:      "browse.sh",
			Kind:      externalSourceKindBrowseSh,
			URL:       defaultBrowseShURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 30,
		},
		{
			Key:       buildSkillSourceID(externalSourceKindHermesIndex, defaultHermesIndexURL),
			Name:      "Hermes Skills Index",
			Kind:      externalSourceKindHermesIndex,
			URL:       defaultHermesIndexURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   false,
			SortOrder: 40,
		},
	}
}

func (s *Service) configuredExternalSkillSources() []externalSkillSource {
	sources := make([]externalSkillSource, 0)
	if s.config.SkillsDefaultSourcesEnabled {
		sources = append(sources, s.defaultExternalSkillSources()...)
	} else if apiURL := strings.TrimRight(strings.TrimSpace(s.config.SkillsAPIURL), "/"); apiURL != "" {
		sources = append(sources, externalSkillSource{
			Key:       buildSkillSourceID(externalSourceKindSkillsSh, apiURL),
			Name:      "skills.sh",
			Kind:      externalSourceKindSkillsSh,
			URL:       apiURL,
			Trust:     externalSourceTrustCommunity,
			Enabled:   true,
			SortOrder: 0,
		})
	}
	for index, raw := range splitExternalSourceList(s.config.SkillsSourceURLs) {
		source, ok := parseConfiguredExternalSource(raw)
		if !ok {
			continue
		}
		source.SortOrder = 100 + index*10
		sources = append(sources, source)
	}
	seen := map[string]struct{}{}
	result := make([]externalSkillSource, 0, len(sources))
	for _, source := range sources {
		if source.Key == "" {
			continue
		}
		if _, ok := seen[source.Key]; ok {
			continue
		}
		seen[source.Key] = struct{}{}
		result = append(result, source)
	}
	sort.SliceStable(result, func(i int, j int) bool {
		if result[i].SortOrder != result[j].SortOrder {
			return result[i].SortOrder < result[j].SortOrder
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func configuredExternalSourceIDs(sources []externalSkillSource) map[string]struct{} {
	result := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		key := strings.TrimSpace(source.Key)
		if key == "" {
			continue
		}
		result[key] = struct{}{}
	}
	return result
}

func (s *Service) externalSkillSources(ctx context.Context) []externalSkillSource {
	configuredSources := s.configuredExternalSkillSources()
	if s.skillStore == nil {
		return configuredSources
	}
	if err := s.ensureConfiguredSkillSources(ctx, configuredSources); err != nil {
		slog.WarnContext(ctx, "初始化 skill 来源配置失败", "err", err)
		return configuredSources
	}
	rows, err := s.skillStore.ListEnabledSources(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		slog.WarnContext(ctx, "读取 skill 来源配置失败", "err", err)
		return configuredSources
	}
	configuredIDs := configuredExternalSourceIDs(configuredSources)
	sources := make([]externalSkillSource, 0, len(rows))
	for _, row := range rows {
		if _, ok := configuredIDs[row.SourceID]; !ok {
			continue
		}
		sources = append(sources, externalSkillSource{
			Key:       row.SourceID,
			Name:      row.Name,
			Kind:      row.Kind,
			URL:       row.URL,
			Trust:     row.Trust,
			Enabled:   row.Enabled,
			SortOrder: row.SortOrder,
		})
	}
	return sources
}

func (s *Service) ensureConfiguredSkillSources(ctx context.Context, sources []externalSkillSource) error {
	if s.skillStore == nil {
		return nil
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	for _, source := range sources {
		if err := s.skillStore.EnsureSource(ctx, skillstore.SourceEntity{
			OwnerUserID: ownerUserID,
			SourceID:    source.Key,
			Name:        source.Name,
			Kind:        source.Kind,
			URL:         source.URL,
			Trust:       firstNonEmpty(source.Trust, externalSourceTrustCommunity),
			Enabled:     source.Enabled,
			SortOrder:   source.SortOrder,
		}); err != nil {
			return err
		}
	}
	return nil
}

func externalSkillSourceInfoFromSource(source externalSkillSource) ExternalSkillSourceInfo {
	return ExternalSkillSourceInfo{
		SourceID:  source.Key,
		Name:      source.Name,
		Kind:      source.Kind,
		URL:       source.URL,
		Trust:     source.Trust,
		Enabled:   source.Enabled,
		SortOrder: source.SortOrder,
	}
}

func externalSkillSourceInfoFromEntity(entity skillstore.SourceEntity) ExternalSkillSourceInfo {
	return ExternalSkillSourceInfo{
		SourceID:      entity.SourceID,
		Name:          entity.Name,
		Kind:          entity.Kind,
		URL:           entity.URL,
		Trust:         entity.Trust,
		Enabled:       entity.Enabled,
		SortOrder:     entity.SortOrder,
		LastCheckedAt: entity.LastCheckedAt,
		LastError:     entity.LastError,
	}
}

func splitExternalSourceList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseConfiguredExternalSource(raw string) (externalSkillSource, bool) {
	label := ""
	sourceURL := strings.TrimSpace(raw)
	if before, after, ok := strings.Cut(sourceURL, "|"); ok {
		label = strings.TrimSpace(before)
		sourceURL = strings.TrimSpace(after)
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return externalSkillSource{}, false
	}
	kind := classifyExternalSourceKind(parsed)
	name := firstNonEmpty(label, externalSourceDefaultName(kind, parsed))
	sourceURL = strings.TrimRight(parsed.String(), "/")
	return externalSkillSource{
		Key:     buildSkillSourceID(kind, sourceURL),
		Name:    name,
		Kind:    kind,
		URL:     sourceURL,
		Trust:   externalSourceTrustCommunity,
		Enabled: true,
	}, true
}

func classifyExternalSourceKind(parsed *url.URL) string {
	path := strings.ToLower(parsed.Path)
	host := strings.ToLower(parsed.Host)
	switch {
	case strings.Contains(host, "claude-plugins.dev"):
		return externalSourceKindClaudePlugins
	case strings.Contains(host, "skills.sh"):
		return externalSourceKindSkillsSh
	case strings.Contains(host, "clawhub.ai"):
		return externalSourceKindClawhub
	case strings.Contains(host, "hermes-agent.nousresearch.com"):
		return externalSourceKindHermesIndex
	case strings.Contains(host, "browse.sh"):
		return externalSourceKindBrowseSh
	}
	if strings.HasSuffix(path, ".json") || strings.Contains(path, ".well-known") {
		return externalSourceKindWellKnown
	}
	if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".zip") {
		return externalSourceKindURL
	}
	if strings.Contains(strings.ToLower(parsed.Host), "github.com") || strings.HasSuffix(path, ".git") {
		return externalSourceKindGit
	}
	return externalSourceKindWellKnown
}

func externalSourceDefaultName(kind string, parsed *url.URL) string {
	if strings.Contains(strings.ToLower(parsed.Host), "github.com") {
		return "GitHub"
	}
	switch kind {
	case externalSourceKindClaudePlugins:
		return "claude-plugins.dev"
	case externalSourceKindSkillsSh:
		return "skills.sh"
	case externalSourceKindClawhub:
		return "clawhub.ai"
	case externalSourceKindHermesIndex:
		return "Hermes Skills Index"
	case externalSourceKindBrowseSh:
		return "browse.sh"
	case externalSourceKindWellKnown:
		return "Skill Index"
	case externalSourceKindURL:
		return "URL"
	case externalSourceKindGit:
		return "Git"
	default:
		return parsed.Host
	}
}

func (s *Service) searchExternalSkillSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	switch source.Kind {
	case externalSourceKindClaudePlugins:
		return s.searchClaudePluginsSource(ctx, source, needle)
	case externalSourceKindSkillsSh:
		return s.searchSkillsShSource(ctx, source, needle)
	case externalSourceKindClawhub:
		return s.searchClawhubSource(ctx, source, needle)
	case externalSourceKindHermesIndex:
		return s.searchHermesIndexSource(ctx, source, needle)
	case externalSourceKindBrowseSh:
		return s.searchBrowseShSource(ctx, source, needle)
	case externalSourceKindWellKnown:
		return s.searchWellKnownSource(ctx, source, needle)
	case externalSourceKindGit, externalSourceKindURL:
		item := externalPointerSourceItem(source)
		if !externalItemMatchesQuery(item, needle) {
			return []ExternalSkillSearchItem{}, nil
		}
		return []ExternalSkillSearchItem{item}, nil
	default:
		return nil, fmt.Errorf("不支持的 skill 来源类型: %s", source.Kind)
	}
}

func (s *Service) searchClaudePluginsSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	requestURL, err := externalSearchURL(source.URL, "/api/skills", map[string]string{
		"q":     needle,
		"limit": fmt.Sprintf("%d", externalSkillSearchLimit(s.config.SkillsAPISearchLimit)),
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude-plugins.dev 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Skills []map[string]any `json:"skills"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("claude-plugins.dev 搜索返回 JSON 解析失败")
	}
	items := make([]ExternalSkillSearchItem, 0, len(payload.Skills))
	for _, row := range payload.Skills {
		name := anyString(row["name"])
		if name == "" {
			continue
		}
		metadata := anyMap(row["metadata"])
		repoOwner := anyString(metadata["repoOwner"])
		repoName := anyString(metadata["repoName"])
		gitPath := anyString(metadata["directoryPath"])
		rawURL := anyString(metadata["rawFileUrl"])
		gitURL := ""
		if repoOwner != "" && repoName != "" {
			gitURL = "https://github.com/" + repoOwner + "/" + repoName
		}
		importMode := externalSourceKindGit
		packageSpec := gitURL
		if gitURL == "" && rawURL != "" {
			importMode = externalSourceKindURL
			packageSpec = rawURL
		}
		if packageSpec == "" {
			continue
		}
		detailURL := firstNonEmpty(anyString(row["sourceUrl"]), githubTreeURL(gitURL, gitPath), rawURL, gitURL)
		items = append(items, ExternalSkillSearchItem{
			Name:           name,
			Title:          firstNonEmpty(anyString(row["title"]), name),
			Description:    firstNonEmpty(anyString(row["description"]), "来自 claude-plugins.dev 的搜索结果"),
			Source:         firstNonEmpty(anyString(row["namespace"]), gitURL, source.URL),
			PackageSpec:    packageSpec,
			SkillSlug:      name,
			Installs:       anyInt(row["installs"]),
			DetailURL:      detailURL,
			ReadmeMarkdown: "",
			SourceKind:     externalSourceKindClaudePlugins,
			SourceKey:      source.Key,
			SourceName:     source.Name,
			SourceTrust:    source.Trust,
			ImportMode:     importMode,
			GitURL:         gitURL,
			GitPath:        gitPath,
			RawURL:         rawURL,
			Tags:           anyStringSlice(row["tags"]),
			Version:        firstNonEmpty(anyString(row["version"]), packageSpec),
		})
	}
	return items, nil
}

func (s *Service) searchSkillsShSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	apiURL := strings.TrimRight(strings.TrimSpace(source.URL), "/")
	requestURL, err := externalSearchURL(apiURL, "/api/search", map[string]string{
		"q":     needle,
		"limit": fmt.Sprintf("%d", externalSkillSearchLimit(s.config.SkillsAPISearchLimit)),
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills.sh 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Skills  []map[string]any `json:"skills"`
		Results []map[string]any `json:"results"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("skills.sh 搜索返回 JSON 解析失败")
	}
	rows := payload.Skills
	if len(rows) == 0 {
		rows = payload.Results
	}
	items := make([]ExternalSkillSearchItem, 0, len(rows))
	for _, row := range rows {
		name := anyString(row["name"])
		slug := firstNonEmpty(anyString(row["id"]), anyString(row["slug"]), name)
		if name == "" || slug == "" {
			continue
		}
		sourceRef := anyString(row["source"])
		item := ExternalSkillSearchItem{
			Name:           name,
			Title:          firstNonEmpty(anyString(row["title"]), name),
			Description:    firstNonEmpty(anyString(row["description"]), "来自 skills.sh 的搜索结果"),
			Source:         sourceRef,
			PackageSpec:    buildSkillsPackageSpec(sourceRef, slug, name),
			SkillSlug:      name,
			Installs:       anyInt(row["installs"]),
			DetailURL:      apiURL + "/" + slug,
			ReadmeMarkdown: "",
			SourceKind:     externalSourceKindSkillsSh,
			SourceKey:      source.Key,
			SourceName:     source.Name,
			SourceTrust:    source.Trust,
			ImportMode:     externalSourceKindSkillsSh,
			Tags:           anyStringSlice(row["tags"]),
			Version:        firstNonEmpty(anyString(row["version"]), buildSkillsPackageSpec(sourceRef, slug, name)),
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) searchClawhubSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	requestURL, err := externalSearchURL(source.URL, "/api/v1/search", map[string]string{"q": needle})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clawhub.ai 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Results []map[string]any `json:"results"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("clawhub.ai 搜索返回 JSON 解析失败")
	}
	items := make([]ExternalSkillSearchItem, 0, len(payload.Results))
	for _, row := range payload.Results {
		slug := anyString(row["slug"])
		if slug == "" {
			continue
		}
		owner := firstNonEmpty(anyString(row["ownerHandle"]), anyString(anyMap(row["owner"])["handle"]))
		name := firstNonEmpty(anyString(row["displayName"]), anyString(row["display_name"]), slug)
		rawURL := clawhubDownloadURL(source.URL, slug)
		if rawURL == "" {
			continue
		}
		stats := anyMap(row["stats"])
		items = append(items, ExternalSkillSearchItem{
			Name:           slug,
			Title:          name,
			Description:    firstNonEmpty(anyString(row["summary"]), anyString(row["description"]), "来自 clawhub.ai 的搜索结果"),
			Source:         firstNonEmpty(owner, source.URL),
			PackageSpec:    rawURL,
			SkillSlug:      slug,
			Installs:       firstNonZero(anyInt(row["downloads"]), anyInt(row["installs"]), anyInt(row["installsAllTime"]), anyInt(stats["downloads"]), anyInt(stats["installsAllTime"])),
			DetailURL:      clawhubDetailURL(source.URL, owner, slug),
			ReadmeMarkdown: "",
			SourceKind:     externalSourceKindClawhub,
			SourceKey:      source.Key,
			SourceName:     source.Name,
			SourceTrust:    source.Trust,
			ImportMode:     externalSourceKindURL,
			RawURL:         rawURL,
			Version:        firstNonEmpty(anyString(row["version"]), slug),
		})
	}
	return items, nil
}

func (s *Service) searchHermesIndexSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Hermes Skills Index 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Skills []map[string]any `json:"skills"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("Hermes Skills Index 返回 JSON 解析失败")
	}
	limit := externalSkillSearchLimit(s.config.SkillsAPISearchLimit)
	items := make([]ExternalSkillSearchItem, 0, limit)
	for _, row := range payload.Skills {
		if len(items) >= limit {
			break
		}
		item := hermesIndexRowItem(source, row)
		if item.Name == "" || item.SkillSlug == "" || item.GitURL == "" || item.GitPath == "" {
			continue
		}
		if !externalItemMatchesQuery(item, needle) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) searchBrowseShSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("browse.sh 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Skills []map[string]any `json:"skills"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("browse.sh 返回 JSON 解析失败")
	}
	limit := externalSkillSearchLimit(s.config.SkillsAPISearchLimit)
	items := make([]ExternalSkillSearchItem, 0, limit)
	for _, row := range payload.Skills {
		if len(items) >= limit {
			break
		}
		item := browseShRowItem(source, row)
		if item.Name == "" || item.SkillSlug == "" || item.RawURL == "" {
			continue
		}
		if !externalItemMatchesQuery(item, needle) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) searchWellKnownSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s 搜索失败: HTTP %d", source.Name, response.StatusCode)
	}
	var payload any
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s 返回 JSON 解析失败", source.Name)
	}
	rows := externalIndexRows(payload)
	items := make([]ExternalSkillSearchItem, 0, len(rows))
	for _, row := range rows {
		item := externalIndexRowItem(source, row)
		if item.SkillSlug == "" || item.Name == "" {
			continue
		}
		if !externalItemMatchesQuery(item, needle) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func externalIndexRows(payload any) []map[string]any {
	switch typed := payload.(type) {
	case []any:
		return anyMapRows(typed)
	case map[string]any:
		for _, key := range []string{"skills", "items", "results"} {
			if rows := anyMapRows(typed[key]); len(rows) > 0 {
				return rows
			}
		}
	}
	return []map[string]any{}
}

func anyMapRows(value any) []map[string]any {
	rawRows, ok := value.([]any)
	if !ok {
		return []map[string]any{}
	}
	rows := make([]map[string]any, 0, len(rawRows))
	for _, raw := range rawRows {
		row, ok := raw.(map[string]any)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func externalIndexRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	name := firstNonEmpty(anyString(row["name"]), anyString(row["id"]), anyString(row["slug"]))
	slug := firstNonEmpty(anyString(row["slug"]), name)
	gitURL := firstNonEmpty(anyString(row["git_url"]), anyString(row["repository_url"]), anyString(row["repo_url"]))
	gitBranch := firstNonEmpty(anyString(row["git_branch"]), anyString(row["branch"]), anyString(row["ref"]))
	gitPath := firstNonEmpty(anyString(row["git_path"]), anyString(row["skill_path"]), anyString(row["path"]))
	rawURL := firstNonEmpty(anyString(row["raw_url"]), anyString(row["skill_url"]), anyString(row["archive_url"]))
	if rawURL == "" && externalURLLooksImportable(anyString(row["url"])) {
		rawURL = anyString(row["url"])
	}
	packageSpec := firstNonEmpty(anyString(row["package_spec"]), gitURL, rawURL, anyString(row["source"]))
	detailURL := firstNonEmpty(anyString(row["detail_url"]), anyString(row["homepage"]), anyString(row["readme_url"]), rawURL, gitURL)
	importMode := normalizeImportMode(firstNonEmpty(
		anyString(row["import_mode"]),
		inferExternalImportMode(ExternalSkillSearchItem{GitURL: gitURL, RawURL: rawURL, PackageSpec: packageSpec}),
	))
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          firstNonEmpty(anyString(row["title"]), name),
		Description:    firstNonEmpty(anyString(row["description"]), anyString(row["summary"]), "来自外部来源的技能"),
		Source:         firstNonEmpty(anyString(row["source"]), source.URL),
		PackageSpec:    packageSpec,
		SkillSlug:      slug,
		Installs:       anyInt(row["installs"]),
		DetailURL:      detailURL,
		ReadmeMarkdown: anyString(row["readme_markdown"]),
		SourceKind:     source.Kind,
		SourceKey:      source.Key,
		SourceName:     source.Name,
		SourceTrust:    source.Trust,
		ImportMode:     importMode,
		GitURL:         gitURL,
		GitBranch:      gitBranch,
		GitPath:        gitPath,
		RawURL:         rawURL,
		Tags:           anyStringSlice(row["tags"]),
		Version:        firstNonEmpty(anyString(row["version"]), packageSpec),
	}
}

func hermesIndexRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	name := firstNonEmpty(anyString(row["name"]), anyString(row["id"]), anyString(row["identifier"]))
	identifier := anyString(row["identifier"])
	gitIdentifier := firstNonEmpty(anyString(row["resolved_github_id"]), githubIdentifierFromRepoPath(anyString(row["repo"]), anyString(row["path"])))
	gitURL, gitPath := splitGitHubIdentifier(gitIdentifier)
	extra := anyMap(row["extra"])
	detailURL := firstNonEmpty(anyString(extra["detail_url"]), githubTreeURL(gitURL, gitPath), anyString(extra["repo_url"]))
	sourceLabel := firstNonEmpty(anyString(row["source"]), source.Name)
	trust := normalizeExternalTrust(firstNonEmpty(anyString(row["trust_level"]), source.Trust))
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          firstNonEmpty(anyString(row["title"]), name),
		Description:    firstNonEmpty(anyString(row["description"]), "来自 Hermes Skills Index 的搜索结果"),
		Source:         identifier,
		PackageSpec:    gitURL,
		SkillSlug:      name,
		Installs:       anyInt(extra["installs"]),
		DetailURL:      detailURL,
		ReadmeMarkdown: "",
		SourceKind:     externalSourceKindHermesIndex,
		SourceKey:      source.Key,
		SourceName:     source.Name + " / " + sourceLabel,
		SourceTrust:    trust,
		ImportMode:     externalSourceKindGit,
		GitURL:         gitURL,
		GitPath:        gitPath,
		Tags:           anyStringSlice(row["tags"]),
		Version:        firstNonEmpty(anyString(row["generated_at"]), gitIdentifier),
	}
}

func browseShRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	slug := anyString(row["slug"])
	name := firstNonEmpty(anyString(row["name"]), anyString(row["task"]), slug)
	title := firstNonEmpty(anyString(row["title"]), name)
	rawURL := githubBlobToRawURL(anyString(row["sourceUrl"]))
	if rawURL == "" && externalURLLooksImportable(anyString(row["skillMdUrl"])) {
		rawURL = anyString(row["skillMdUrl"])
	}
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          title,
		Description:    firstNonEmpty(anyString(row["description"]), "来自 browse.sh 的网站自动化技能"),
		Source:         firstNonEmpty(anyString(row["hostname"]), anyString(row["source"]), source.URL),
		PackageSpec:    rawURL,
		SkillSlug:      firstNonEmpty(slug, name),
		Installs:       anyInt(row["installCount"]),
		DetailURL:      firstNonEmpty(rawURL, anyString(row["sourceUrl"])),
		ReadmeMarkdown: "",
		SourceKind:     externalSourceKindBrowseSh,
		SourceKey:      source.Key,
		SourceName:     source.Name,
		SourceTrust:    source.Trust,
		ImportMode:     externalSourceKindURL,
		RawURL:         rawURL,
		Tags:           anyStringSlice(row["tags"]),
		Version:        firstNonEmpty(anyString(row["updated"]), rawURL),
	}
}

func externalPointerSourceItem(source externalSkillSource) ExternalSkillSearchItem {
	name := skillNameFromSourceURL(source.URL)
	item := ExternalSkillSearchItem{
		Name:        name,
		Title:       name,
		Description: "来自 " + source.Name + " 的外部技能来源",
		Source:      source.URL,
		PackageSpec: source.URL,
		SkillSlug:   name,
		DetailURL:   source.URL,
		SourceKind:  source.Kind,
		SourceKey:   source.Key,
		SourceName:  source.Name,
		SourceTrust: source.Trust,
		ImportMode:  source.Kind,
		Version:     source.URL,
	}
	if source.Kind == externalSourceKindGit {
		item.GitURL = source.URL
	} else {
		item.RawURL = source.URL
	}
	return item
}

func skillNameFromSourceURL(sourceURL string) string {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "external-skill"
	}
	name := strings.TrimSuffix(filepath.Base(parsed.Path), ".git")
	name = strings.TrimSuffix(name, ".zip")
	name = strings.TrimSuffix(name, ".md")
	if strings.EqualFold(name, "SKILL") || name == "." || name == "/" || name == "" {
		segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for i := len(segments) - 1; i >= 0; i-- {
			if strings.TrimSpace(segments[i]) != "" && !strings.EqualFold(segments[i], "skills") {
				return strings.TrimSpace(segments[i])
			}
		}
		return parsed.Host
	}
	return name
}

func externalItemMatchesQuery(item ExternalSkillSearchItem, needle string) bool {
	query := strings.ToLower(strings.TrimSpace(needle))
	if query == "" {
		return true
	}
	values := []string{item.Name, item.Title, item.Description, item.Source, item.PackageSpec, item.SourceName}
	values = append(values, item.Tags...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func externalSearchURL(sourceURL string, defaultPath string, queryValues map[string]string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("skill 来源 URL 不正确")
	}
	if strings.Trim(parsed.Path, "/") == "" {
		parsed.Path = defaultPath
	}
	values := parsed.Query()
	for key, value := range queryValues {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func externalSkillSearchLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	return limit
}

func githubTreeURL(gitURL string, gitPath string) string {
	if strings.TrimSpace(gitURL) == "" || strings.TrimSpace(gitPath) == "" {
		return strings.TrimSpace(gitURL)
	}
	return strings.TrimRight(strings.TrimSpace(gitURL), "/") + "/tree/main/" + strings.Trim(strings.TrimSpace(gitPath), "/")
}

func githubIdentifierFromRepoPath(repo string, skillPath string) string {
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	skillPath = strings.Trim(strings.TrimSpace(skillPath), "/")
	if repo == "" || skillPath == "" {
		return ""
	}
	return repo + "/" + skillPath
}

func splitGitHubIdentifier(identifier string) (string, string) {
	parts := strings.SplitN(strings.Trim(strings.TrimSpace(identifier), "/"), "/", 3)
	if len(parts) < 3 {
		return "", ""
	}
	repo := parts[0] + "/" + parts[1]
	return "https://github.com/" + repo, parts[2]
}

func githubBlobToRawURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Host, "github.com") {
		return ""
	}
	parts := strings.SplitN(strings.Trim(parsed.Path, "/"), "/", 5)
	if len(parts) < 5 || parts[2] != "blob" {
		return ""
	}
	return "https://raw.githubusercontent.com/" + parts[0] + "/" + parts[1] + "/" + parts[3] + "/" + parts[4]
}

func normalizeExternalTrust(trust string) string {
	switch strings.ToLower(strings.TrimSpace(trust)) {
	case "official", "builtin", "trusted":
		return externalSourceTrustOfficial
	case "private":
		return externalSourceTrustPrivate
	default:
		return externalSourceTrustCommunity
	}
}

func clawhubDownloadURL(sourceURL string, slug string) string {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Host == "" || strings.TrimSpace(slug) == "" {
		return ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, "/search") {
		path = strings.TrimSuffix(path, "/search")
	}
	if path == "" || path == "/" || !strings.Contains(path, "/api/") {
		path = "/api/v1"
	}
	parsed.Path = strings.TrimRight(path, "/") + "/download"
	values := parsed.Query()
	values.Set("slug", strings.TrimSpace(slug))
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func clawhubDetailURL(sourceURL string, owner string, slug string) string {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed.Host == "" {
		return "https://clawhub.ai/skills/" + strings.TrimSpace(slug)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if strings.TrimSpace(owner) != "" {
		parsed.Path = "/" + strings.Trim(strings.TrimSpace(owner), "/") + "/" + strings.Trim(strings.TrimSpace(slug), "/")
	} else {
		parsed.Path = "/skills/" + strings.Trim(strings.TrimSpace(slug), "/")
	}
	return parsed.String()
}

func inferExternalImportMode(item ExternalSkillSearchItem) string {
	if strings.TrimSpace(item.GitURL) != "" {
		return externalSourceKindGit
	}
	if strings.TrimSpace(item.RawURL) != "" || externalURLLooksImportable(item.DetailURL) {
		return externalSourceKindURL
	}
	if strings.TrimSpace(item.PackageSpec) != "" {
		return externalSourceKindSkillsSh
	}
	return ""
}

func normalizeImportMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "claude-plugins", "claude_plugins", "claude-plugins.dev":
		return externalSourceKindClaudePlugins
	case "skills.sh", "skillssh", "skills_sh":
		return externalSourceKindSkillsSh
	case "clawhub", "clawhub.ai":
		return externalSourceKindClawhub
	case "hermes", "hermes-index", "hermes_index":
		return externalSourceKindHermesIndex
	case "browse.sh", "browsesh", "browse_sh", "browse-sh":
		return externalSourceKindBrowseSh
	case "github", "git":
		return externalSourceKindGit
	case "direct", "direct_url", "url", "zip":
		return externalSourceKindURL
	default:
		return strings.TrimSpace(mode)
	}
}

func externalURLLooksImportable(rawURL string) bool {
	path := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".zip")
}

func (s *Service) validateExternalURL(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", errors.New("skills 外部链接非法")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("skills 外部链接协议非法")
	}
	if parsed.Host == "" {
		return "", errors.New("skills 外部链接域名为空")
	}
	allowedHosts := map[string]struct{}{
		"claude-plugins.dev":        {},
		"skills.sh":                 {},
		"clawhub.ai":                {},
		"github.com":                {},
		"raw.githubusercontent.com": {},
	}
	for _, source := range s.externalSkillSources(ctx) {
		sourceURL, parseErr := url.Parse(source.URL)
		if parseErr == nil && sourceURL.Host != "" {
			allowedHosts[strings.ToLower(sourceURL.Host)] = struct{}{}
		}
	}
	if _, ok := allowedHosts[strings.ToLower(parsed.Host)]; !ok {
		return "", errors.New("skills 外部链接域名未在来源白名单中")
	}
	return parsed.String(), nil
}

func extractPreviewMarkdown(html string) string {
	trimmed := strings.TrimSpace(html)
	marker := `"dangerouslySetInnerHTML":{"__html":"`
	if !strings.Contains(html, marker) {
		if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "# ") || strings.Contains(trimmed, "\n# ") {
			return trimmed
		}
		if !strings.Contains(strings.ToLower(trimmed), "<html") && strings.Contains(trimmed, "\n") {
			return trimmed
		}
		return ""
	}
	_, fragment, _ := strings.Cut(html, marker)
	fragment, _, _ = strings.Cut(fragment, `"}}`)
	decoded := strings.ReplaceAll(fragment, `\n`, "\n")
	decoded = strings.ReplaceAll(decoded, `\"`, `"`)
	result := decoded
	for _, item := range previewMarkdownRules {
		result = item.pattern.ReplaceAllString(result, item.replace)
	}
	result = strings.ReplaceAll(result, "&#x3C;", "<")
	result = strings.ReplaceAll(result, "&quot;", `"`)
	return strings.TrimSpace(result)
}

func dedupeExternalItems(items []ExternalSkillSearchItem) []ExternalSkillSearchItem {
	seen := map[string]struct{}{}
	result := make([]ExternalSkillSearchItem, 0, len(items))
	for _, item := range items {
		key := firstNonEmpty(item.SourceKey, item.PackageSpec, item.GitURL, item.RawURL, item.DetailURL) + "::" + firstNonEmpty(item.SkillSlug, item.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func anyMap(value any) map[string]any {
	item, _ := value.(map[string]any)
	if item == nil {
		return map[string]any{}
	}
	return item
}

func anyString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func anyStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return normalizeStringSlice(typed)
	case []any:
		items := make([]string, 0, len(typed))
		for _, raw := range typed {
			if text, ok := raw.(string); ok {
				items = append(items, text)
			}
		}
		return normalizeStringSlice(items)
	case string:
		if strings.TrimSpace(typed) == "" {
			return []string{}
		}
		return normalizeStringSlice(strings.Split(typed, ","))
	default:
		return []string{}
	}
}

func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func anyInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func cleanSkillSubdirPath(skillPath string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(skillPath), "/")
	if trimmed == "" {
		return "", nil
	}
	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if cleaned == "." {
		return "", nil
	}
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", errors.New("skill 子目录非法")
	}
	return cleaned, nil
}
