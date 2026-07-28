package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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

	if output, runErr := s.cloneGitRepository(ctx, repositoryURL, tempDir, gitCloneOptions{
		Branch:            strings.TrimSpace(branch),
		CleanGlobalConfig: shouldUseCleanGitConfigForRepository(repositoryURL, manifest),
	}); runErr != nil {
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
	commitOutput, revErr := s.runCommandWithEnv(ctx, tempDir, nil, "git", "rev-parse", "HEAD")
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
	manifest.Version = firstNonEmpty(commitOutput, manifest.Version, "git")
	return s.importSourceDir(ctx, sourceDir, manifest)
}

// ImportSkillsSh 从 skills.sh 搜索结果导入技能。
func (s *Service) ImportSkillsSh(ctx context.Context, packageSpec string, skillSlug string) (*Detail, error) {
	target, err := parseSkillsShImportTarget(packageSpec, skillSlug)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "nexus-skills-sh-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	if output, runErr := s.cloneGitRepository(ctx, target.RepositoryURL, tempDir, gitCloneOptions{
		CleanGlobalConfig: shouldUseCleanGitConfigForRepository(target.RepositoryURL, externalManifest{SourceKind: externalSourceKindSkillsSh}),
	}); runErr != nil {
		return nil, fmt.Errorf("skills.sh Git 导入失败: %s", output)
	}
	sourceDir, err := findSkillsShSourceDir(tempDir, target.SkillPath, target.SkillSlug)
	if err != nil {
		return nil, err
	}
	relativeSourceDir, relErr := filepath.Rel(tempDir, sourceDir)
	if relErr == nil && relativeSourceDir != "." {
		target.SkillPath = filepath.ToSlash(relativeSourceDir)
	}
	commitOutput, revErr := s.runCommandWithEnv(ctx, tempDir, nil, "git", "rev-parse", "HEAD")
	if revErr != nil {
		slog.WarnContext(ctx, "skills.sh git rev-parse HEAD 失败", "repository_url", target.RepositoryURL, "err", revErr)
	}
	return s.importSourceDir(ctx, sourceDir, externalManifest{
		SourceType:  sourceTypeExternal,
		SourceRef:   target.Identifier,
		SourceKind:  externalSourceKindSkillsSh,
		SourceKey:   firstNonEmpty(s.config.SkillsAPIURL, "https://skills.sh"),
		SourceName:  "skills.sh",
		SourceTrust: externalSourceTrustCommunity,
		ImportMode:  "skills_sh",
		GitURL:      target.RepositoryURL,
		GitPath:     filepath.ToSlash(target.SkillPath),
		GitCommit:   strings.TrimSpace(commitOutput),
		DetailURL:   skillsShDetailURL(firstNonEmpty(s.config.SkillsAPIURL, defaultSkillsShURL), target.SourceRef, target.SkillSlug),
		Version:     firstNonEmpty(commitOutput, target.Identifier),
	})
}

// ImportExternalSkill 按搜索结果携带的来源信息导入技能。
func (s *Service) ImportExternalSkill(ctx context.Context, item ExternalSkillSearchItem) (*Detail, error) {
	mode := normalizeImportMode(firstNonEmpty(item.ImportMode, inferExternalImportMode(item)))
	manifest := externalManifest{
		Name:           externalItemSkillName(item),
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

func externalItemSkillName(item ExternalSkillSearchItem) string {
	for _, candidate := range []string{
		item.SkillSlug,
		item.Name,
		skillNameFromSourceURL(firstNonEmpty(item.RawURL, item.PackageSpec, item.DetailURL, item.GitURL)),
	} {
		if name := normalizeSkillNameFallback(candidate); name != "" {
			return name
		}
	}
	return ""
}

func normalizeSkillNameFallback(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		trimmed = skillNameFromSourceURL(trimmed)
	}
	trimmed = strings.Trim(strings.ReplaceAll(trimmed, "\\", "/"), "/")
	if trimmed == "" {
		return ""
	}
	name := filepath.Base(filepath.FromSlash(trimmed))
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimSuffix(name, ".zip")
	name = strings.TrimSuffix(name, ".md")
	if name == "." || name == string(os.PathSeparator) || strings.EqualFold(name, "SKILL") {
		return ""
	}
	return strings.TrimSpace(name)
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

	if isZipPayload(targetURL, response.Header.Get("Content-Type"), body) {
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
	manifest.Name = firstNonEmpty(normalizeSkillNameFallback(manifest.Name), skillNameFromSourceURL(targetURL))
	manifest.SourceTrust = firstNonEmpty(manifest.SourceTrust, externalSourceTrustCommunity)
	manifest.ImportMode = externalSourceKindURL
	manifest.RawURL = targetURL
	manifest.Version = firstNonEmpty(manifest.Version, targetURL)
	return s.importSourceDir(ctx, sourceDir, manifest)
}

func (s *Service) importSourceDir(ctx context.Context, sourceDir string, manifest externalManifest) (*Detail, error) {
	content, skillMDPath, skillName, err := readSkillSource(sourceDir)
	if err != nil {
		return nil, err
	}
	parsed := parseSkillFrontmatter(content, firstNonEmpty(manifest.Name, skillName))
	parsed.Name = strings.TrimSpace(parsed.Name)
	if parsed.Name == "" {
		return nil, errors.New("SKILL.md 缺少 name")
	}
	if err = validateSkillName(parsed.Name); err != nil {
		return nil, err
	}
	if err = s.ensureExternalSkillNameAvailable(ctx, parsed.Name); err != nil {
		return nil, err
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	if err = workspacesvc.EnsureUserSkillLibrary(s.config, ownerUserID); err != nil {
		return nil, err
	}
	root := s.registryRoot(ctx)
	targetDir := filepath.Join(root, parsed.Name)
	boundaryRoot := workspacesvc.UserSkillLibraryRoot(s.config, ownerUserID)
	boundaryFS, err := workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		boundaryRoot,
		true,
	)
	if err != nil {
		return nil, err
	}
	defer boundaryFS.Close()
	stagingRelative, err := boundaryFS.MkdirTemp(privateSkillStagingRoot, ".external-skill-", 0o700)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = boundaryFS.RemoveAll(stagingRelative)
	}()
	if err = copyDirectoryAt(filepath.Dir(skillMDPath), boundaryFS, stagingRelative); err != nil {
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
	if err = writeSkillDirectoryFileAt(boundaryFS, stagingRelative, ".nexus-skill.json", payload, 0o644); err != nil {
		return nil, err
	}
	targetRelative, err := relativeSkillPath(boundaryFS, targetDir)
	if err != nil {
		return nil, err
	}
	if err = workspacesvc.ReplaceDirectoryAt(boundaryFS, stagingRelative, targetRelative); err != nil {
		return nil, err
	}
	if err = workspacesvc.RefreshUserSkillLibrary(s.config, ownerUserID); err != nil {
		return nil, err
	}
	if err = s.upsertImportedSkillRecordAt(ctx, boundaryFS, targetDir, manifest, parsed); err != nil {
		removeErr := boundaryFS.RemoveAll(targetRelative)
		refreshErr := workspacesvc.RefreshUserSkillLibrary(s.config, ownerUserID)
		return nil, errors.Join(err, removeErr, refreshErr)
	}
	return s.GetSkillDetail(ctx, parsed.Name, "")
}

func (s *Service) ensureExternalSkillNameAvailable(ctx context.Context, name string) error {
	trimmed := strings.TrimSpace(name)
	if containsSkillName(systemSkillNames, trimmed) {
		return errors.New("系统 Skill 名称不能被外部来源覆盖")
	}
	if containsSkillName(internalSkillNames, trimmed) {
		return errors.New("内部 Skill 名称不能被外部来源覆盖")
	}
	for _, root := range builtinSearchRootsForContext(ctx, projectRoot(), s.config.AppMode) {
		exists, searchErr := skillNameExists(root, trimmed)
		if searchErr != nil {
			return searchErr
		}
		if exists {
			return errors.New("已有本地 Skill 使用该名称，外部来源不能覆盖")
		}
	}
	ownerRoot, err := s.openOwnerSkillLibrary(ctx, true)
	if err != nil {
		return err
	}
	defer ownerRoot.Close()
	confinedRoot, entries, err := readSkillRegistryDirectoriesAt(ownerRoot, true)
	if err != nil {
		return err
	}
	defer confinedRoot.Close()
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), trimmed) && entry.Name() != trimmed {
			return errors.New("已有外部 Skill 使用仅大小写不同的名称")
		}
	}
	return nil
}

func containsSkillName(names map[string]struct{}, target string) bool {
	for name := range names {
		if strings.EqualFold(name, target) {
			return true
		}
	}
	return false
}

func skillNameExists(root string, target string) (bool, error) {
	confinedRoot, err := confinedfs.Open(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer confinedRoot.Close()
	entries, err := fs.ReadDir(confinedRoot.FS(), ".")
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), target) {
			continue
		}
		skillRoot, openErr := confinedRoot.OpenRootNoSymlink(entry.Name())
		if os.IsNotExist(openErr) || errors.Is(openErr, confinedfs.ErrSymlink) {
			continue
		}
		if openErr != nil {
			return false, openErr
		}
		skillFile, fileErr := skillRoot.OpenFileNoSymlink("SKILL.md", os.O_RDONLY, 0)
		skillRoot.Close()
		if fileErr == nil {
			_ = skillFile.Close()
			return true, nil
		}
		if !os.IsNotExist(fileErr) {
			return false, fileErr
		}
	}
	return false, nil
}

func (s *Service) readManifest(skillDir string) (externalManifest, error) {
	payload, err := readSkillDirectoryFile(skillDir, ".nexus-skill.json")
	if err != nil {
		return externalManifest{}, err
	}
	var manifest externalManifest
	if err = json.Unmarshal(payload, &manifest); err != nil {
		return externalManifest{}, err
	}
	return manifest, nil
}
