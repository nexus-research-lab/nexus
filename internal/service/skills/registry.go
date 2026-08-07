package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	workspacesvc "github.com/nexus-research-lab/nexus/internal/service/workspace"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) loadExternalRecords(ctx context.Context) (map[string]catalogRecord, error) {
	ownerUserID := authctx.OwnerUserID(ctx)
	if err := workspacesvc.EnsureUserSkillLibrary(s.config, ownerUserID); err != nil {
		return nil, err
	}
	ownerRoot, err := s.openOwnerSkillLibrary(ctx, false)
	if err != nil {
		return nil, err
	}
	defer ownerRoot.Close()
	root := s.registryRoot(ctx)
	if s.skillStore != nil {
		if err := s.backfillImportedSkillRecords(ctx, root, ownerRoot); err != nil {
			return nil, err
		}
		return s.loadExternalRecordsFromDB(ctx, root, ownerRoot)
	}
	return s.loadExternalRecordsFromRootAt(root, ownerRoot)
}

func externalOriginKind(sourceKind string) string {
	switch strings.TrimSpace(sourceKind) {
	case externalSourceKindClaudePlugins,
		externalSourceKindSkillsSh,
		externalSourceKindClawhub,
		externalSourceKindHermesIndex,
		externalSourceKindBrowseSh,
		externalSourceKindWellKnown:
		return originKindMarketplace
	default:
		return originKindUserImport
	}
}

func (s *Service) loadExternalRecordsFromDB(
	ctx context.Context,
	root string,
	ownerRoot *confinedfs.Root,
) (map[string]catalogRecord, error) {
	records, err := s.skillStore.ListImportedSkills(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	result := map[string]catalogRecord{}
	names := map[string]struct{}{}
	for _, record := range records {
		if validateSkillName(record.SkillName) != nil {
			continue
		}
		item := s.buildExternalRecordFromEntity(root, ownerRoot, record)
		addCatalogRecord(result, names, item)
	}
	return result, nil
}

func (s *Service) buildExternalRecordFromEntity(
	root string,
	ownerRoot *confinedfs.Root,
	record skillstore.ImportedSkillEntity,
) catalogRecord {
	skillDir := filepath.Join(root, record.SkillName)
	contentBytes, err := readSkillRegistryFileAt(ownerRoot, record.SkillName, "SKILL.md")
	parsed := parseSkillFrontmatter("", record.SkillName)
	if err == nil {
		parsed = parseSkillFrontmatter(string(contentBytes), record.SkillName)
	}
	tags := jsoncodec.ParseStringSlice(record.TagsJSON)
	if tags == nil {
		tags = []string{}
	}
	manifest := externalManifest{
		Name:           record.SkillName,
		Title:          record.Title,
		Description:    record.Description,
		Scope:          record.Scope,
		Tags:           tags,
		CategoryKey:    record.CategoryKey,
		CategoryName:   record.CategoryName,
		Version:        record.Version,
		SourceType:     sourceTypeExternal,
		SourceRef:      record.SourceRef,
		SourceKind:     record.SourceKind,
		SourceKey:      record.SourceID,
		SourceName:     record.SourceName,
		SourceTrust:    record.SourceTrust,
		ImportMode:     record.ImportMode,
		Recommendation: record.Recommendation,
		GitURL:         record.GitURL,
		GitBranch:      record.GitBranch,
		GitPath:        record.GitPath,
		GitCommit:      record.GitCommit,
		RawURL:         record.RawURL,
		DetailURL:      record.DetailURL,
	}
	detail := Detail{
		Info: Info{
			Name:         firstNonEmpty(record.SkillName, parsed.Name),
			Title:        firstNonEmpty(record.Title, parsed.Title, record.SkillName),
			Description:  firstNonEmpty(record.Description, parsed.Description),
			Scope:        defaultSkillScope(firstNonEmpty(record.Scope, parsed.Scope)),
			Tags:         firstNonEmptySlice(tags, parsed.Tags),
			CategoryKey:  firstNonEmpty(record.CategoryKey, parsed.CategoryKey, "custom-imports"),
			CategoryName: firstNonEmpty(record.CategoryName, parsed.CategoryName, "自定义导入"),
			SourceType:   sourceTypeExternal,
			SourceRef:    firstNonEmpty(record.SourceRef, skillDir),
			Version:      firstNonEmpty(record.Version, parsed.Version, "external"),
			Locked:       false,
			HasUpdate:    record.UpdateAvailable,
			Deletable:    true,
			SourceKind:   record.SourceKind,
			SourceName:   record.SourceName,
			SourceTrust:  record.SourceTrust,
			ImportMode:   record.ImportMode,
			LastError:    record.LastError,
			StorageScope: storageScopeUserGlobal,
			OriginKind:   externalOriginKind(record.SourceKind),
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: firstNonEmpty(record.Recommendation, parsed.Recommendation, "外部导入能力。"),
	}
	return catalogRecord{Detail: detail, SourcePath: skillDir, Manifest: manifest}
}

func (s *Service) backfillImportedSkillRecords(
	ctx context.Context,
	root string,
	ownerRoot *confinedfs.Root,
) error {
	fileRecords, err := s.loadExternalRecordsFromRootAt(root, ownerRoot)
	if err != nil {
		return err
	}
	for _, record := range fileRecords {
		if existing, getErr := s.skillStore.GetImportedSkill(ctx, authctx.OwnerUserID(ctx), record.Detail.Name); getErr != nil {
			return getErr
		} else if existing != nil {
			continue
		}
		manifest, readErr := readManifestAt(ownerRoot, record.Detail.Name)
		if readErr != nil {
			continue
		}
		parsed := frontmatterData{
			Name:           record.Detail.Name,
			Title:          record.Detail.Title,
			Description:    record.Detail.Description,
			Scope:          record.Detail.Scope,
			Tags:           record.Detail.Tags,
			Version:        record.Detail.Version,
			CategoryKey:    record.Detail.CategoryKey,
			CategoryName:   record.Detail.CategoryName,
			Recommendation: record.Detail.Recommendation,
			ReadmeMarkdown: record.Detail.ReadmeMarkdown,
		}
		if err = s.upsertImportedSkillRecordAt(
			ctx,
			ownerRoot,
			record.SourcePath,
			manifest,
			parsed,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) upsertImportedSkillRecord(ctx context.Context, skillDir string, manifest externalManifest, parsed frontmatterData) error {
	return s.upsertImportedSkillRecordWithHash(
		ctx,
		skillDir,
		manifest,
		parsed,
		hashSkillContent(skillDir),
	)
}

func (s *Service) upsertImportedSkillRecordAt(
	ctx context.Context,
	ownerRoot *confinedfs.Root,
	skillPath string,
	manifest externalManifest,
	parsed frontmatterData,
) error {
	relativePath, err := relativeSkillPath(ownerRoot, skillPath)
	if err != nil {
		return err
	}
	payload, err := readSkillFileAtOwnerPath(ownerRoot, skillPath, "SKILL.md")
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	return s.upsertImportedSkillRecordWithHash(
		ctx,
		filepath.Join(ownerRoot.Name(), filepath.FromSlash(relativePath)),
		manifest,
		parsed,
		hex.EncodeToString(sum[:]),
	)
}

func (s *Service) upsertImportedSkillRecordWithHash(
	ctx context.Context,
	skillDir string,
	manifest externalManifest,
	parsed frontmatterData,
	contentHash string,
) error {
	if s.skillStore == nil {
		return nil
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	now := time.Now().UTC()
	canonicalName := filepath.Base(filepath.Clean(skillDir))
	entity := skillstore.ImportedSkillEntity{
		OwnerUserID:    ownerUserID,
		SkillName:      canonicalName,
		Title:          firstNonEmpty(manifest.Title, parsed.Title, parsed.Name, canonicalName),
		Description:    firstNonEmpty(manifest.Description, parsed.Description),
		Scope:          defaultSkillScope(firstNonEmpty(manifest.Scope, parsed.Scope)),
		TagsJSON:       jsoncodec.MarshalStringSlice(firstNonEmptySlice(manifest.Tags, parsed.Tags)),
		CategoryKey:    firstNonEmpty(manifest.CategoryKey, parsed.CategoryKey, "custom-imports"),
		CategoryName:   firstNonEmpty(manifest.CategoryName, parsed.CategoryName, "自定义导入"),
		Recommendation: firstNonEmpty(manifest.Recommendation, parsed.Recommendation, "外部导入能力。"),
		Version:        firstNonEmpty(manifest.Version, parsed.Version, "external"),
		SourceID:       s.importedSkillSourceID(manifest),
		SourceKind:     manifest.SourceKind,
		SourceRef:      manifest.SourceRef,
		SourceName:     manifest.SourceName,
		SourceTrust:    firstNonEmpty(manifest.SourceTrust, externalSourceTrustCommunity),
		ImportMode:     manifest.ImportMode,
		GitURL:         manifest.GitURL,
		GitBranch:      manifest.GitBranch,
		GitPath:        manifest.GitPath,
		GitCommit:      manifest.GitCommit,
		RawURL:         manifest.RawURL,
		DetailURL:      manifest.DetailURL,
		ContentHash:    contentHash,
		LastImportedAt: &now,
	}
	return s.skillStore.UpsertImportedSkill(ctx, entity)
}

func (s *Service) importedSkillSourceID(manifest externalManifest) string {
	sourceKey := strings.TrimSpace(manifest.SourceKey)
	if strings.HasPrefix(sourceKey, "skill_src_") {
		return sourceKey
	}
	sourceURL := firstNonEmpty(manifest.GitURL, manifest.RawURL, manifest.DetailURL)
	if sourceURL == "" && strings.HasPrefix(strings.TrimSpace(manifest.SourceRef), "http") {
		sourceURL = manifest.SourceRef
	}
	if sourceURL == "" && manifest.ImportMode == "skills_sh" {
		sourceURL = firstNonEmpty(s.config.SkillsAPIURL, "https://skills.sh")
	}
	if sourceURL == "" {
		return ""
	}
	return buildSkillSourceID(firstNonEmpty(manifest.SourceKind, manifest.ImportMode), sourceURL)
}

func hashSkillContent(skillDir string) string {
	payload, err := readSkillDirectoryFile(skillDir, "SKILL.md")
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func buildSkillSourceID(kind string, sourceURL string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(kind) + "\x00" + strings.TrimSpace(sourceURL)))
	return "skill_src_" + hex.EncodeToString(sum[:10])
}

func (s *Service) loadExternalRecordsFromRoot(root string) (map[string]catalogRecord, error) {
	confinedRoot, _, err := readSkillRegistryDirectories(root)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	return loadExternalRecordsFromRegistryRoot(root, confinedRoot)
}

func (s *Service) loadExternalRecordsFromRootAt(
	root string,
	ownerRoot *confinedfs.Root,
) (map[string]catalogRecord, error) {
	confinedRoot, _, err := readSkillRegistryDirectoriesAt(ownerRoot, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	return loadExternalRecordsFromRegistryRoot(root, confinedRoot)
}

func loadExternalRecordsFromRegistryRoot(
	root string,
	confinedRoot *confinedfs.Root,
) (map[string]catalogRecord, error) {
	result := map[string]catalogRecord{}
	names := map[string]struct{}{}
	entries, err := fs.ReadDir(confinedRoot.FS(), ".")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		info, statErr := confinedRoot.Lstat(entry.Name())
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		skillRoot, openErr := confinedRoot.OpenRootNoSymlink(entry.Name())
		if openErr != nil {
			continue
		}
		skillDir := filepath.Join(root, entry.Name())
		payload, readErr := readConfinedRegularFile(skillRoot, ".nexus-skill.json")
		if readErr != nil {
			skillRoot.Close()
			continue
		}
		var manifest externalManifest
		if json.Unmarshal(payload, &manifest) != nil {
			skillRoot.Close()
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(manifest.SourceType), sourceTypeExternal) {
			skillRoot.Close()
			continue
		}
		contentBytes, sourceErr := readConfinedRegularFile(skillRoot, "SKILL.md")
		skillRoot.Close()
		if sourceErr != nil {
			continue
		}
		content := string(contentBytes)
		skillName := entry.Name()
		parsed := parseSkillFrontmatter(content, skillName)
		// owner 全局根与 runtime 一样以直接子目录作 canonical name。
		// manifest/frontmatter 可以提供标题，不能让 Catalog 与真实发现名分叉。
		canonicalName := skillName
		if validateSkillName(canonicalName) != nil {
			continue
		}
		manifest.Name = canonicalName
		detail := Detail{
			Info: Info{
				Name:         canonicalName,
				Title:        firstNonEmpty(manifest.Title, parsed.Title, parsed.Name, skillName),
				Description:  firstNonEmpty(manifest.Description, parsed.Description),
				Scope:        defaultSkillScope(firstNonEmpty(manifest.Scope, parsed.Scope)),
				Tags:         firstNonEmptySlice(manifest.Tags, parsed.Tags),
				CategoryKey:  firstNonEmpty(manifest.CategoryKey, parsed.CategoryKey, "custom-imports"),
				CategoryName: firstNonEmpty(manifest.CategoryName, parsed.CategoryName, "自定义导入"),
				SourceType:   sourceTypeExternal,
				SourceRef:    firstNonEmpty(manifest.SourceRef, skillDir),
				Version:      firstNonEmpty(manifest.Version, parsed.Version, "external"),
				Locked:       false,
				HasUpdate:    false,
				Deletable:    true,
				StorageScope: storageScopeUserGlobal,
				OriginKind:   externalOriginKind(manifest.SourceKind),
			},
			ReadmeMarkdown: parsed.ReadmeMarkdown,
			Recommendation: firstNonEmpty(manifest.Recommendation, parsed.Recommendation, "外部导入能力。"),
		}
		addCatalogRecord(
			result,
			names,
			catalogRecord{Detail: detail, SourcePath: skillDir, Manifest: manifest},
		)
	}
	return result, nil
}

func (s *Service) registryRoot(ctx context.Context) string {
	return s.registryRootForOwner(authctx.OwnerUserID(ctx))
}

func (s *Service) openOwnerSkillLibrary(
	ctx context.Context,
	create bool,
) (*confinedfs.Root, error) {
	ownerUserID := authctx.OwnerUserID(ctx)
	return workspacestore.New(s.config.WorkspacePath).OpenOwnerWorkspacePath(
		ownerUserID,
		workspacesvc.UserSkillLibraryRoot(s.config, ownerUserID),
		create,
	)
}

func (s *Service) registryRootForOwner(ownerUserID string) string {
	return workspacesvc.UserSkillDiscoveryRoot(s.config, ownerUserID)
}

func readManifestAt(ownerRoot *confinedfs.Root, skillName string) (externalManifest, error) {
	payload, err := readSkillDirectoryFileAt(ownerRoot, skillName, ".nexus-skill.json")
	if err != nil {
		return externalManifest{}, err
	}
	var manifest externalManifest
	if err = json.Unmarshal(payload, &manifest); err != nil {
		return externalManifest{}, err
	}
	return manifest, nil
}

func readSkillManifestAtOwnerPath(
	ownerRoot *confinedfs.Root,
	skillPath string,
) (externalManifest, error) {
	payload, err := readSkillFileAtOwnerPath(ownerRoot, skillPath, ".nexus-skill.json")
	if err != nil {
		return externalManifest{}, err
	}
	var manifest externalManifest
	if err = json.Unmarshal(payload, &manifest); err != nil {
		return externalManifest{}, err
	}
	return manifest, nil
}
