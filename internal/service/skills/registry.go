package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

func (s *Service) loadExternalRecords(ctx context.Context) (map[string]catalogRecord, error) {
	if err := s.ensureLegacyRegistryMigrated(ctx); err != nil {
		return nil, err
	}
	root := s.registryRoot(ctx)
	if s.skillStore != nil {
		if err := s.backfillImportedSkillRecords(ctx, root); err != nil {
			return nil, err
		}
		return s.loadExternalRecordsFromDB(ctx, root)
	}
	return s.loadExternalRecordsFromRoot(root)
}

func (s *Service) loadExternalRecordsFromDB(ctx context.Context, root string) (map[string]catalogRecord, error) {
	records, err := s.skillStore.ListImportedSkills(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	result := map[string]catalogRecord{}
	for _, record := range records {
		item := s.buildExternalRecordFromEntity(root, record)
		result[item.Detail.Name] = item
	}
	return result, nil
}

func (s *Service) buildExternalRecordFromEntity(root string, record skillstore.ImportedSkillEntity) catalogRecord {
	skillDir := filepath.Join(root, record.SkillName)
	content, _, fallbackName, err := readSkillSource(skillDir)
	parsed := parseSkillFrontmatter("", record.SkillName)
	if err == nil {
		parsed = parseSkillFrontmatter(content, fallbackName)
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
		},
		ReadmeMarkdown: parsed.ReadmeMarkdown,
		Recommendation: firstNonEmpty(record.Recommendation, parsed.Recommendation, "外部导入能力。"),
	}
	return catalogRecord{Detail: detail, SourcePath: skillDir, Manifest: manifest}
}

func (s *Service) backfillImportedSkillRecords(ctx context.Context, root string) error {
	fileRecords, err := s.loadExternalRecordsFromRoot(root)
	if err != nil {
		return err
	}
	for _, record := range fileRecords {
		if existing, getErr := s.skillStore.GetImportedSkill(ctx, authctx.OwnerUserID(ctx), record.Detail.Name); getErr != nil {
			return getErr
		} else if existing != nil {
			continue
		}
		manifest, readErr := s.readManifest(record.SourcePath)
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
		if err = s.upsertImportedSkillRecord(ctx, record.SourcePath, manifest, parsed); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) upsertImportedSkillRecord(ctx context.Context, skillDir string, manifest externalManifest, parsed frontmatterData) error {
	if s.skillStore == nil {
		return nil
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	now := time.Now().UTC()
	entity := skillstore.ImportedSkillEntity{
		OwnerUserID:    ownerUserID,
		SkillName:      firstNonEmpty(manifest.Name, parsed.Name, filepath.Base(skillDir)),
		Title:          firstNonEmpty(manifest.Title, parsed.Title, parsed.Name),
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
		ContentHash:    hashSkillContent(skillDir),
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
	payload, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
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
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	result := map[string]catalogRecord{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(root, entry.Name())
		payload, readErr := os.ReadFile(filepath.Join(skillDir, ".nexus-skill.json"))
		if readErr != nil {
			continue
		}
		var manifest externalManifest
		if json.Unmarshal(payload, &manifest) != nil {
			continue
		}
		content, _, skillName, sourceErr := readSkillSource(skillDir)
		if sourceErr != nil {
			continue
		}
		parsed := parseSkillFrontmatter(content, skillName)
		detail := Detail{
			Info: Info{
				Name:         firstNonEmpty(manifest.Name, parsed.Name),
				Title:        firstNonEmpty(manifest.Title, parsed.Title, skillName),
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
			},
			ReadmeMarkdown: parsed.ReadmeMarkdown,
			Recommendation: firstNonEmpty(manifest.Recommendation, parsed.Recommendation, "外部导入能力。"),
		}
		result[detail.Name] = catalogRecord{Detail: detail, SourcePath: skillDir, Manifest: manifest}
	}
	return result, nil
}

func (s *Service) registryBaseRoot() string {
	base := strings.TrimSpace(s.config.CacheFileDir)
	if base == "" {
		base = "cache"
	}
	return filepath.Join(base, "skills", "registry")
}

func (s *Service) registryRoot(ctx context.Context) string {
	return s.registryRootForOwner(authctx.OwnerUserID(ctx))
}

func (s *Service) registryRootForOwner(ownerUserID string) string {
	return filepath.Join(s.registryBaseRoot(), registryUsersDirName, safeRegistrySegment(ownerUserID))
}

func safeRegistrySegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return authctx.SystemUserID
	}
	var builder strings.Builder
	for _, item := range trimmed {
		switch {
		case item >= 'a' && item <= 'z':
			builder.WriteRune(item)
		case item >= 'A' && item <= 'Z':
			builder.WriteRune(item)
		case item >= '0' && item <= '9':
			builder.WriteRune(item)
		case item == '-' || item == '_' || item == '.' || item == '@':
			builder.WriteRune(item)
		default:
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return authctx.SystemUserID
	}
	return builder.String()
}
