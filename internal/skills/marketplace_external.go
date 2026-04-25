package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ExternalSkillSearchItem 表示外部技能搜索结果。
type ExternalSkillSearchItem struct {
	Name           string `json:"name"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Source         string `json:"source"`
	PackageSpec    string `json:"package_spec"`
	SkillSlug      string `json:"skill_slug"`
	Installs       int    `json:"installs"`
	DetailURL      string `json:"detail_url"`
	ReadmeMarkdown string `json:"readme_markdown"`
}

// SearchExternalSkillsResponse 表示外部技能搜索响应。
type SearchExternalSkillsResponse struct {
	Query   string                    `json:"query"`
	Results []ExternalSkillSearchItem `json:"results"`
}

// ExternalSkillPreviewResponse 表示技能详情预览。
type ExternalSkillPreviewResponse struct {
	DetailURL      string `json:"detail_url"`
	ReadmeMarkdown string `json:"readme_markdown"`
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
func (s *Service) ImportUploadedArchive(filename string, payload []byte) (*Detail, error) {
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
	return s.importSourceDir(sourceDir, externalManifest{
		SourceType: sourceTypeExternal,
		SourceRef:  strings.TrimSpace(filename),
		ImportMode: "uploaded_zip",
		Version:    "uploaded",
	})
}

// ImportGit 从 Git 仓库导入技能。
func (s *Service) ImportGit(ctx context.Context, repositoryURL string, branch string) (*Detail, error) {
	repositoryURL = strings.TrimSpace(repositoryURL)
	if repositoryURL == "" {
		return nil, errors.New("url 不能为空")
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
	sourceDir, err := findSkillSourceDir(tempDir)
	if err != nil {
		return nil, err
	}
	commitOutput, _ := s.runCommand(ctx, tempDir, "git", "rev-parse", "HEAD")
	return s.importSourceDir(sourceDir, externalManifest{
		SourceType: sourceTypeExternal,
		SourceRef:  repositoryURL,
		ImportMode: "git",
		GitURL:     repositoryURL,
		GitBranch:  strings.TrimSpace(branch),
		GitCommit:  strings.TrimSpace(commitOutput),
		Version:    firstNonEmpty(strings.TrimSpace(commitOutput), "git"),
	})
}

// SearchExternalSkills 搜索 skills.sh 外部技能。
func (s *Service) SearchExternalSkills(ctx context.Context, query string, includeReadme bool) (*SearchExternalSkillsResponse, error) {
	needle := strings.TrimSpace(query)
	if needle == "" {
		return &SearchExternalSkillsResponse{Query: "", Results: []ExternalSkillSearchItem{}}, nil
	}
	apiURL := strings.TrimRight(strings.TrimSpace(s.config.SkillsAPIURL), "/")
	if apiURL == "" {
		return nil, errors.New("skills.sh API 地址为空")
	}
	requestURL := apiURL + "/api/search?q=" + url.QueryEscape(needle) + "&limit=" + fmt.Sprintf("%d", s.config.SkillsAPISearchLimit)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
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
		source := anyString(row["source"])
		detailURL := strings.TrimRight(apiURL, "/") + "/" + slug
		item := ExternalSkillSearchItem{
			Name:           name,
			Title:          name,
			Description:    firstNonEmpty(anyString(row["description"]), "来自 skills.sh 的搜索结果"),
			Source:         source,
			PackageSpec:    buildSkillsPackageSpec(source, slug, name),
			SkillSlug:      name,
			Installs:       anyInt(row["installs"]),
			DetailURL:      detailURL,
			ReadmeMarkdown: "",
		}
		if includeReadme {
			preview, previewErr := s.GetExternalSkillPreview(ctx, detailURL)
			if previewErr == nil {
				item.ReadmeMarkdown = preview.ReadmeMarkdown
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Installs != items[j].Installs {
			return items[i].Installs > items[j].Installs
		}
		return items[i].Name < items[j].Name
	})
	return &SearchExternalSkillsResponse{Query: needle, Results: dedupeExternalItems(items)}, nil
}

// GetExternalSkillPreview 获取 skills.sh 详情页的预览。
func (s *Service) GetExternalSkillPreview(ctx context.Context, detailURL string) (*ExternalSkillPreviewResponse, error) {
	targetURL, err := validateExternalDetailURL(strings.TrimSpace(s.config.SkillsAPIURL), detailURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills 预览加载失败: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return &ExternalSkillPreviewResponse{
		DetailURL:      targetURL,
		ReadmeMarkdown: extractPreviewMarkdown(string(body)),
	}, nil
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
	return s.importSourceDir(sourceDir, externalManifest{
		SourceType: sourceTypeExternal,
		SourceRef:  packageSpec,
		ImportMode: "skills_sh",
		Version:    packageSpec,
	})
}

// UpdateImportedSkills 更新所有已导入的外部技能。
func (s *Service) UpdateImportedSkills(ctx context.Context) (*UpdateInstalledSkillsResponse, error) {
	records, err := s.loadExternalRecords()
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
		if _, updateErr := s.UpdateSingleSkill(ctx, name); updateErr != nil {
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
	records, err := s.loadExternalRecords()
	if err != nil {
		return nil, err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return nil, errors.New("skill not found")
	}
	manifest, err := s.readManifest(record.SourcePath)
	if err != nil {
		return nil, err
	}
	switch manifest.ImportMode {
	case "git":
		return s.ImportGit(ctx, manifest.GitURL, manifest.GitBranch)
	case "skills_sh":
		return s.ImportSkillsSh(ctx, manifest.SourceRef, manifest.Name)
	default:
		return nil, errors.New("该 skill 来源不支持更新")
	}
}

func (s *Service) importSourceDir(sourceDir string, manifest externalManifest) (*Detail, error) {
	content, skillMDPath, skillName, err := readSkillSource(sourceDir)
	if err != nil {
		return nil, err
	}
	parsed := parseSkillFrontmatter(content, skillName)
	if parsed.Name == "" {
		return nil, errors.New("SKILL.md 缺少 name")
	}
	targetDir := filepath.Join(s.registryRoot(), parsed.Name)
	if err = os.RemoveAll(targetDir); err != nil {
		return nil, err
	}
	if err = copyDirectory(filepath.Dir(skillMDPath), targetDir); err != nil {
		return nil, err
	}
	manifest.Name = parsed.Name
	manifest.Title = firstNonEmpty(manifest.Title, parsed.Title, parsed.Name)
	manifest.Description = firstNonEmpty(manifest.Description, parsed.Description)
	manifest.Scope = defaultSkillScope(firstNonEmpty(manifest.Scope, parsed.Scope))
	manifest.Tags = firstNonEmptySlice(manifest.Tags, parsed.Tags)
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
	return s.GetSkillDetail(context.Background(), parsed.Name, "")
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
	if len(command) == 0 {
		return "", errors.New("命令不能为空")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}
	cmd.Env = os.Environ()
	if strings.TrimSpace(s.config.SkillsAPIURL) != "" {
		cmd.Env = append(cmd.Env, "SKILLS_API_URL="+strings.TrimSpace(s.config.SkillsAPIURL))
	}
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func (s *Service) runPnpmCommand(ctx context.Context, workDir string, args ...string) (string, error) {
	command := []string{"pnpm"}
	if registry := strings.TrimSpace(s.config.PnpmRegistry); registry != "" {
		command = append(command, "--registry", registry)
	}
	if cacheRoot := strings.TrimSpace(s.config.CacheFileDir); cacheRoot != "" {
		storeDir := filepath.Join(cacheRoot, "pnpm-store")
		_ = os.MkdirAll(storeDir, 0o755)
		command = append(command, "--store-dir", storeDir)
	}
	command = append(command, args...)
	return s.runCommand(ctx, workDir, command...)
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
		data, readErr := io.ReadAll(readerHandle)
		_ = readerHandle.Close()
		if readErr != nil {
			return readErr
		}
		if err = os.WriteFile(cleanTarget, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func findSkillSourceDir(root string) (string, error) {
	var matches []string
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
		matches = append(matches, filepath.Dir(path))
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", errors.New("未找到 SKILL.md")
	}
	sort.Slice(matches, func(i int, j int) bool {
		return len(matches[i]) < len(matches[j])
	})
	return matches[0], nil
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

func validateExternalDetailURL(apiBase string, detailURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(detailURL))
	if err != nil {
		return "", errors.New("skills 预览链接非法")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("skills 预览链接协议非法")
	}
	allowedHosts := map[string]struct{}{"skills.sh": {}}
	if baseURL, parseErr := url.Parse(strings.TrimSpace(apiBase)); parseErr == nil && baseURL.Host != "" {
		allowedHosts[baseURL.Host] = struct{}{}
	}
	if _, ok := allowedHosts[parsed.Host]; !ok {
		return "", errors.New("skills 预览链接域名非法")
	}
	return parsed.String(), nil
}

func extractPreviewMarkdown(html string) string {
	marker := `"dangerouslySetInnerHTML":{"__html":"`
	if !strings.Contains(html, marker) {
		return ""
	}
	fragment := strings.SplitN(html, marker, 2)[1]
	fragment = strings.SplitN(fragment, `"}}`, 2)[0]
	decoded := strings.ReplaceAll(fragment, `\n`, "\n")
	decoded = strings.ReplaceAll(decoded, `\"`, `"`)
	replacements := []struct {
		pattern string
		replace string
	}{
		{`<pre.*?><code.*?>`, "```text\n"},
		{`</code></pre>`, "\n```"},
		{`<h1>(.*?)</h1>`, "# $1\n"},
		{`<h2>(.*?)</h2>`, "## $1\n"},
		{`<h3>(.*?)</h3>`, "### $1\n"},
		{`<li>(.*?)</li>`, "- $1"},
		{`<p>(.*?)</p>`, "$1\n"},
		{`<[^>]+>`, ""},
	}
	result := decoded
	for _, item := range replacements {
		result = regexp.MustCompile(item.pattern).ReplaceAllString(result, item.replace)
	}
	result = strings.ReplaceAll(result, "&#x3C;", "<")
	result = strings.ReplaceAll(result, "&quot;", `"`)
	return strings.TrimSpace(result)
}

func dedupeExternalItems(items []ExternalSkillSearchItem) []ExternalSkillSearchItem {
	seen := map[string]struct{}{}
	result := make([]ExternalSkillSearchItem, 0, len(items))
	for _, item := range items {
		key := item.PackageSpec + "::" + item.SkillSlug
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func anyString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
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
