// INPUT: owner-scoped private registry requests、来源凭据与 owner catalog version。
// OUTPUT: 经远端验证的私有来源 CRUD、搜索/导入与 reconcile-aware 结果。
// POS: skills 私有 registry 边界；网络校验在短事务外完成，功能写入在 catalog CAS 事务内提交。
package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

const maxPrivateRegistryIndexBytes = 2 * 1024 * 1024

var privateRegistryHTTPClient = &http.Client{
	Timeout: 20 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type privateRegistrySkill struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Version        string   `json:"version"`
	Tags           []string `json:"tags"`
	DownloadURL    string   `json:"download_url"`
	SHA256         string   `json:"sha256"`
	Size           int64    `json:"size"`
	ReadmeMarkdown string   `json:"readme_markdown"`
}

type privateRegistryResponse struct {
	Skills []privateRegistrySkill `json:"skills"`
	Total  int                    `json:"total"`
}

// CreateExternalSkillSource 新增并验证一个用户私有来源。
func (s *Service) CreateExternalSkillSource(
	ctx context.Context,
	request CreateExternalSkillSourceRequest,
) (*ExternalSkillSourceInfo, error) {
	return s.createExternalSkillSource(ctx, request, nil)
}

// CreateExternalSkillSourceAtVersion 仅在 owner catalog version 匹配时新增私有来源。
func (s *Service) CreateExternalSkillSourceAtVersion(
	ctx context.Context,
	request CreateExternalSkillSourceRequest,
	expectedVersion int64,
) (*ExternalSkillSourceInfo, error) {
	return s.createExternalSkillSource(ctx, request, &expectedVersion)
}

func (s *Service) createExternalSkillSource(
	ctx context.Context,
	request CreateExternalSkillSourceRequest,
	expectedVersion *int64,
) (*ExternalSkillSourceInfo, error) {
	if s.skillStore == nil {
		return nil, errors.New("skill source store not configured")
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return nil, errors.New("来源名称不能为空")
	}
	if len([]rune(name)) > 255 {
		return nil, errors.New("来源名称过长")
	}
	baseURL, err := s.validatePrivateRegistryBaseURL(ctx, request.URL)
	if err != nil {
		return nil, err
	}
	authType, err := normalizePrivateSourceAuthType(request.AuthType)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(request.Token)
	if authType == externalSourceAuthBearer && token == "" {
		return nil, errors.New("Bearer Token 不能为空")
	}
	sourceID := buildSkillSourceID(externalSourceKindPrivateRegistry, baseURL)
	ownerUserID := authctx.OwnerUserID(ctx)
	remoteSource := externalSkillSource{
		Key:        sourceID,
		Name:       name,
		Kind:       externalSourceKindPrivateRegistry,
		URL:        baseURL,
		Trust:      externalSourceTrustPrivate,
		Enabled:    true,
		SortOrder:  1000,
		ManagedBy:  externalSourceManagedByUser,
		AuthType:   authType,
		Credential: token,
	}
	if _, err = s.queryPrivateRegistry(ctx, remoteSource, "", "", 1); err != nil {
		return nil, fmt.Errorf("验证私有来源失败: %w", err)
	}
	// ponytail: 暂沿用旧列明文存储，统一 SecretStore 落地后替换。
	entity := skillstore.SourceEntity{
		OwnerUserID:          ownerUserID,
		SourceID:             sourceID,
		Name:                 name,
		Kind:                 externalSourceKindPrivateRegistry,
		URL:                  baseURL,
		Trust:                externalSourceTrustPrivate,
		ManagedBy:            externalSourceManagedByUser,
		AuthType:             authType,
		CredentialsEncrypted: token,
		Enabled:              true,
		SortOrder:            1000,
	}
	checkedAt := time.Now().UTC()
	_, err = s.withCatalogMutation(
		ctx,
		expectedVersion,
		true,
		func(mutation *skillstore.CatalogMutation) error {
			existing, loadErr := mutation.GetSource(ctx, sourceID)
			if loadErr != nil {
				return loadErr
			}
			if existing != nil {
				return errors.New("该私有来源已存在")
			}
			if upsertErr := mutation.UpsertSource(ctx, entity); upsertErr != nil {
				return upsertErr
			}
			return mutation.RecordSourceCheck(ctx, sourceID, checkedAt, "")
		},
	)
	if err != nil {
		return nil, err
	}
	return s.readPrivateSkillSourceAfterMutation(ctx, ownerUserID, sourceID)
}

func (s *Service) updatePrivateSkillSource(
	ctx context.Context,
	existing skillstore.SourceEntity,
	request ExternalSkillSourceRequest,
	expectedVersion *int64,
) (*ExternalSkillSourceInfo, error) {
	if request.Name == nil && request.Enabled == nil && request.AuthType == nil && request.Token == nil {
		return nil, errors.New("来源更新至少要提供一个字段")
	}
	entity, token, authChanged, err := s.preparePrivateSkillSourceUpdate(existing, request)
	if err != nil {
		return nil, err
	}
	if authChanged {
		source := externalSkillSource{
			Key:        entity.SourceID,
			Name:       entity.Name,
			Kind:       entity.Kind,
			URL:        entity.URL,
			Trust:      entity.Trust,
			Enabled:    entity.Enabled,
			SortOrder:  entity.SortOrder,
			ManagedBy:  externalSourceManagedByUser,
			AuthType:   entity.AuthType,
			Credential: token,
		}
		if _, err = s.queryPrivateRegistry(ctx, source, "", "", 1); err != nil {
			return nil, fmt.Errorf("验证私有来源失败: %w", err)
		}
	}
	checkedAt := time.Now().UTC()
	_, err = s.withCatalogMutation(
		ctx,
		expectedVersion,
		true,
		func(mutation *skillstore.CatalogMutation) error {
			current, loadErr := mutation.GetSource(ctx, existing.SourceID)
			if loadErr != nil {
				return loadErr
			}
			if current == nil {
				return errors.New("skill source not found")
			}
			if sourceManagedBy(*current) != externalSourceManagedByUser ||
				current.Kind != externalSourceKindPrivateRegistry {
				return errors.New("仅用户私有来源可修改")
			}
			if !samePrivateSkillSourceConfiguration(*current, existing) {
				return ErrCatalogSnapshotUnstable
			}
			if upsertErr := mutation.UpsertSource(ctx, entity); upsertErr != nil {
				return upsertErr
			}
			if authChanged {
				return mutation.RecordSourceCheck(ctx, entity.SourceID, checkedAt, "")
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return s.readPrivateSkillSourceAfterMutation(
		ctx,
		entity.OwnerUserID,
		entity.SourceID,
	)
}

func (s *Service) preparePrivateSkillSourceUpdate(
	existing skillstore.SourceEntity,
	request ExternalSkillSourceRequest,
) (skillstore.SourceEntity, string, bool, error) {
	entity := existing
	if request.Name != nil {
		entity.Name = strings.TrimSpace(*request.Name)
		if entity.Name == "" {
			return skillstore.SourceEntity{}, "", false, errors.New("来源名称不能为空")
		}
		if len([]rune(entity.Name)) > 255 {
			return skillstore.SourceEntity{}, "", false, errors.New("来源名称过长")
		}
	}
	if request.Enabled != nil {
		entity.Enabled = *request.Enabled
	}
	authChanged := request.AuthType != nil || request.Token != nil
	authType := firstNonEmpty(entity.AuthType, externalSourceAuthNone)
	if request.AuthType != nil {
		var normalizeErr error
		authType, normalizeErr = normalizePrivateSourceAuthType(*request.AuthType)
		if normalizeErr != nil {
			return skillstore.SourceEntity{}, "", false, normalizeErr
		}
	}
	token := ""
	if authChanged {
		if authType == externalSourceAuthBearer {
			if request.Token != nil && strings.TrimSpace(*request.Token) != "" {
				token = strings.TrimSpace(*request.Token)
			} else {
				token = strings.TrimSpace(entity.CredentialsEncrypted)
			}
			if token == "" {
				return skillstore.SourceEntity{}, "", false, errors.New("Bearer Token 不能为空")
			}
		}
		entity.AuthType = authType
		entity.CredentialsEncrypted = token
		entity.LastError = ""
	}
	return entity, token, authChanged, nil
}

func samePrivateSkillSourceConfiguration(left skillstore.SourceEntity, right skillstore.SourceEntity) bool {
	return strings.TrimSpace(left.OwnerUserID) == strings.TrimSpace(right.OwnerUserID) &&
		strings.TrimSpace(left.SourceID) == strings.TrimSpace(right.SourceID) &&
		strings.TrimSpace(left.Name) == strings.TrimSpace(right.Name) &&
		strings.TrimSpace(left.Kind) == strings.TrimSpace(right.Kind) &&
		strings.TrimSpace(left.URL) == strings.TrimSpace(right.URL) &&
		strings.TrimSpace(left.Trust) == strings.TrimSpace(right.Trust) &&
		strings.TrimSpace(left.ManagedBy) == strings.TrimSpace(right.ManagedBy) &&
		strings.TrimSpace(left.AuthType) == strings.TrimSpace(right.AuthType) &&
		strings.TrimSpace(left.CredentialsEncrypted) == strings.TrimSpace(right.CredentialsEncrypted) &&
		left.Enabled == right.Enabled &&
		left.SortOrder == right.SortOrder
}

func (s *Service) readPrivateSkillSourceAfterMutation(
	ctx context.Context,
	ownerUserID string,
	sourceID string,
) (*ExternalSkillSourceInfo, error) {
	stored, err := s.skillStore.GetSource(ctx, ownerUserID, sourceID)
	if err != nil || stored == nil {
		if err == nil {
			err = errors.New("skill source not found")
		}
		return nil, &CatalogReconcileError{applied: true, cause: err}
	}
	item := externalSkillSourceInfoFromEntity(*stored)
	return &item, nil
}

// DeleteExternalSkillSource 删除一个用户私有来源，不删除已经导入的 Skill。
func (s *Service) DeleteExternalSkillSource(ctx context.Context, sourceID string) error {
	return s.deleteExternalSkillSource(ctx, sourceID, nil)
}

// DeleteExternalSkillSourceAtVersion 仅在 owner catalog version 匹配时删除私有来源。
func (s *Service) DeleteExternalSkillSourceAtVersion(
	ctx context.Context,
	sourceID string,
	expectedVersion int64,
) error {
	return s.deleteExternalSkillSource(ctx, sourceID, &expectedVersion)
}

func (s *Service) deleteExternalSkillSource(
	ctx context.Context,
	sourceID string,
	expectedVersion *int64,
) error {
	if s.skillStore == nil {
		return errors.New("skill source store not configured")
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	sourceID = strings.TrimSpace(sourceID)
	_, err := s.withCatalogMutation(
		ctx,
		expectedVersion,
		true,
		func(mutation *skillstore.CatalogMutation) error {
			existing, loadErr := mutation.GetSource(ctx, sourceID)
			if loadErr != nil {
				return loadErr
			}
			if existing == nil {
				return errors.New("skill source not found")
			}
			if sourceManagedBy(*existing) != externalSourceManagedByUser ||
				existing.Kind != externalSourceKindPrivateRegistry {
				return errors.New("仅用户私有来源可删除")
			}
			return mutation.DeleteSource(ctx, sourceID)
		},
	)
	if err != nil {
		return err
	}
	stored, readErr := s.skillStore.GetSource(ctx, ownerUserID, sourceID)
	if readErr != nil {
		return &CatalogReconcileError{applied: true, cause: readErr}
	}
	if stored != nil {
		return &CatalogReconcileError{
			applied: true,
			cause:   errors.New("skill source remained after deletion"),
		}
	}
	return nil
}

func (s *Service) searchPrivateRegistrySource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	rows, err := s.queryPrivateRegistry(
		ctx,
		source,
		strings.TrimSpace(needle),
		"",
		privateRegistrySearchLimit(s.config.SkillsAPISearchLimit),
	)
	if err != nil {
		return nil, err
	}
	items := make([]ExternalSkillSearchItem, 0, len(rows))
	for _, row := range rows {
		item := privateRegistrySearchItem(source, row)
		if externalItemMatchesQuery(item, needle) {
			items = append(items, item)
		}
	}
	return items, nil
}

func privateRegistrySearchLimit(configured int) int {
	limit := externalSkillSearchLimit(configured)
	if limit > 100 {
		return 100
	}
	return limit
}

// ImportPrivateSkillFromSource 从服务端重新解析来源记录，避免信任浏览器提交的下载地址。
func (s *Service) ImportPrivateSkillFromSource(ctx context.Context, request ImportPrivateSkillRequest) (*Detail, error) {
	return s.importPrivateSkillFromSource(ctx, request, nil)
}

// ImportPrivateSkillFromSourceAtVersion 仅在 owner catalog version 匹配时发布私有 Skill。
func (s *Service) ImportPrivateSkillFromSourceAtVersion(
	ctx context.Context,
	request ImportPrivateSkillRequest,
	expectedVersion int64,
) (*Detail, error) {
	return s.importPrivateSkillFromSource(ctx, request, &expectedVersion)
}

func (s *Service) importPrivateSkillFromSource(
	ctx context.Context,
	request ImportPrivateSkillRequest,
	expectedVersion *int64,
) (*Detail, error) {
	source, err := s.privateSkillSource(ctx, request.SourceID)
	if err != nil {
		return nil, err
	}
	return s.importPrivateRegistrySkill(ctx, source, request.SkillID, "", expectedVersion)
}

func (s *Service) importPrivateRegistrySkill(
	ctx context.Context,
	source externalSkillSource,
	skillID string,
	expectedName string,
	expectedVersion *int64,
) (*Detail, error) {
	row, err := s.privateRegistrySkillByID(ctx, source, skillID)
	if err != nil {
		return nil, err
	}
	if expectedName != "" && row.Name != expectedName {
		return nil, errors.New("私有来源 skill name 已变更，无法更新")
	}
	payload, err := s.downloadPrivateRegistrySkill(ctx, source, row)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "nexus-private-skill-*")
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
	if err = validatePrivateRegistrySkillSource(tempDir, sourceDir, row.Name); err != nil {
		return nil, err
	}
	return s.importSourceDirAtVersion(ctx, sourceDir, externalManifest{
		Name:           row.Name,
		Title:          firstNonEmpty(row.Title, row.Name),
		Description:    row.Description,
		Tags:           normalizeStringSlice(row.Tags),
		Version:        row.Version,
		SourceType:     sourceTypeExternal,
		SourceRef:      row.ID,
		SourceKind:     externalSourceKindPrivateRegistry,
		SourceKey:      source.Key,
		SourceName:     source.Name,
		SourceTrust:    externalSourceTrustPrivate,
		SourceSkillID:  row.ID,
		ArtifactSHA256: row.SHA256,
		ImportMode:     externalSourceKindPrivateRegistry,
		Recommendation: firstNonEmpty(row.Description, "私有来源导入能力。"),
		RawURL:         row.DownloadURL,
		DetailURL:      source.URL,
	}, expectedVersion)
}

func validatePrivateRegistrySkillSource(root string, sourceDir string, expectedName string) error {
	skillFiles := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillFiles++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if skillFiles != 1 {
		return errors.New("私有 skill 压缩包必须且只能包含一个 SKILL.md")
	}
	content, _, _, err := readSkillSource(sourceDir)
	if err != nil {
		return err
	}
	parsed := parseSkillFrontmatter(content, "")
	if strings.TrimSpace(parsed.Name) != strings.TrimSpace(expectedName) {
		return errors.New("私有 skill 的 SKILL.md name 与索引不一致")
	}
	relativeSourceDir, err := filepath.Rel(root, sourceDir)
	if err != nil || relativeSourceDir == "." || filepath.Dir(relativeSourceDir) != "." {
		return errors.New("私有 skill 必须放在压缩包的单个顶层目录中")
	}
	if filepath.Base(relativeSourceDir) != strings.TrimSpace(expectedName) {
		return errors.New("私有 skill 的顶层目录名与索引 name 不一致")
	}
	return nil
}

func (s *Service) privateRegistrySkillByID(ctx context.Context, source externalSkillSource, skillID string) (privateRegistrySkill, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return privateRegistrySkill{}, errors.New("skill_id 不能为空")
	}
	rows, err := s.queryPrivateRegistry(ctx, source, "", skillID, 1)
	if err != nil {
		return privateRegistrySkill{}, err
	}
	if len(rows) > 1 {
		return privateRegistrySkill{}, errors.New("私有来源精确查询返回了多条记录")
	}
	if len(rows) == 1 && rows[0].ID == skillID {
		return rows[0], nil
	}
	return privateRegistrySkill{}, errors.New("私有来源中未找到该 skill")
}

func (s *Service) checkPrivateRegistrySkillUpdate(ctx context.Context, manifest externalManifest) (bool, error) {
	source, err := s.privateSkillSource(ctx, manifest.SourceKey)
	if err != nil {
		return false, err
	}
	row, err := s.privateRegistrySkillByID(ctx, source, firstNonEmpty(manifest.SourceSkillID, manifest.SourceRef))
	if err != nil {
		return false, err
	}
	if row.Name != manifest.Name {
		return false, errors.New("私有来源 skill name 已变更，无法更新")
	}
	currentHash := strings.TrimSpace(manifest.ArtifactSHA256)
	if currentHash == "" {
		return strings.TrimSpace(row.Version) != "" && strings.TrimSpace(row.Version) != strings.TrimSpace(manifest.Version), nil
	}
	return !strings.EqualFold(currentHash, row.SHA256), nil
}

func (s *Service) downloadPrivateRegistrySkill(ctx context.Context, source externalSkillSource, row privateRegistrySkill) ([]byte, error) {
	request, err := privateRegistryRequest(ctx, source, http.MethodGet, row.DownloadURL)
	if err != nil {
		return nil, err
	}
	response, err := privateRegistryHTTPClient.Do(request)
	if err != nil {
		return nil, privateRegistryTransportError(ctx, "私有 skill 下载")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("私有 skill 下载失败: HTTP %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxExternalImportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxExternalImportBytes {
		return nil, errors.New("私有 skill 压缩包超过大小限制")
	}
	if row.Size > 0 && int64(len(payload)) != row.Size {
		return nil, errors.New("私有 skill 压缩包大小与索引不一致")
	}
	sum := sha256.Sum256(payload)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), row.SHA256) {
		return nil, errors.New("私有 skill 压缩包校验失败")
	}
	if !isZipPayload(row.DownloadURL, response.Header.Get("Content-Type"), payload) {
		return nil, errors.New("私有 skill 下载内容必须是 zip 包")
	}
	return payload, nil
}

func (s *Service) queryPrivateRegistry(ctx context.Context, source externalSkillSource, query string, skillID string, limit int) ([]privateRegistrySkill, error) {
	baseURL, err := s.validatePrivateRegistryBaseURL(ctx, source.URL)
	if err != nil {
		return nil, err
	}
	source.URL = baseURL
	requestURL, err := privateRegistryEndpoint(source.URL, query, skillID, limit)
	if err != nil {
		return nil, err
	}
	request, err := privateRegistryRequest(ctx, source, http.MethodGet, requestURL)
	if err != nil {
		return nil, err
	}
	response, err := privateRegistryHTTPClient.Do(request)
	if err != nil {
		return nil, privateRegistryTransportError(ctx, "私有来源请求")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("私有来源搜索失败: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPrivateRegistryIndexBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPrivateRegistryIndexBytes {
		return nil, errors.New("私有来源索引超过大小限制")
	}
	var payload privateRegistryResponse
	if err = json.Unmarshal(body, &payload); err != nil {
		return nil, errors.New("私有来源返回 JSON 解析失败")
	}
	if payload.Skills == nil {
		return nil, errors.New("私有来源返回 JSON 缺少 skills 数组")
	}
	if limit > 0 && len(payload.Skills) > limit {
		return nil, errors.New("私有来源返回的 skill 数量超过 limit")
	}
	items := make([]privateRegistrySkill, 0, len(payload.Skills))
	for _, raw := range payload.Skills {
		item, normalizeErr := normalizePrivateRegistrySkill(source.URL, raw)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		items = append(items, item)
	}
	return items, nil
}

func privateRegistryTransportError(ctx context.Context, action string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New(action + "失败")
}

func normalizePrivateRegistrySkill(baseURL string, item privateRegistrySkill) (privateRegistrySkill, error) {
	item.ID = strings.TrimSpace(item.ID)
	item.Name = strings.TrimSpace(item.Name)
	item.Title = strings.TrimSpace(item.Title)
	item.Description = strings.TrimSpace(item.Description)
	item.Version = strings.TrimSpace(item.Version)
	item.Tags = normalizeStringSlice(item.Tags)
	item.SHA256 = strings.ToLower(strings.TrimSpace(item.SHA256))
	if item.ID == "" || item.Name == "" {
		return privateRegistrySkill{}, errors.New("私有来源 skill 缺少 id 或 name")
	}
	if len([]rune(item.ID)) > 255 {
		return privateRegistrySkill{}, errors.New("私有来源 skill id 过长")
	}
	if err := validateSkillName(item.Name); err != nil {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 name 不正确: %w", item.ID, err)
	}
	if item.Title == "" || item.Description == "" || item.Version == "" {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 缺少 title、description 或 version", item.ID)
	}
	if len(item.Tags) > 32 {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 tags 超过限制", item.ID)
	}
	if len([]byte(item.ReadmeMarkdown)) > maxExternalPreviewBytes {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 readme_markdown 超过限制", item.ID)
	}
	if len(item.SHA256) != sha256.Size*2 {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 sha256 不正确", item.ID)
	}
	if _, err := hex.DecodeString(item.SHA256); err != nil {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 sha256 不正确", item.ID)
	}
	if item.Size < 0 || item.Size > maxExternalImportBytes {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 size 超过限制", item.ID)
	}
	downloadURL, err := resolvePrivateRegistryURL(baseURL, item.DownloadURL)
	if err != nil {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 download_url 不正确: %w", item.ID, err)
	}
	parsedDownloadURL, err := url.Parse(downloadURL)
	if err != nil || parsedDownloadURL.RawQuery != "" || parsedDownloadURL.ForceQuery {
		return privateRegistrySkill{}, fmt.Errorf("私有来源 skill %s 的 download_url 不能包含查询参数", item.ID)
	}
	item.DownloadURL = downloadURL
	return item, nil
}

func privateRegistrySearchItem(source externalSkillSource, row privateRegistrySkill) ExternalSkillSearchItem {
	return ExternalSkillSearchItem{
		Name:           row.Name,
		Title:          firstNonEmpty(row.Title, row.Name),
		Description:    row.Description,
		Source:         source.URL,
		PackageSpec:    row.ID,
		SkillSlug:      row.Name,
		DetailURL:      source.URL,
		ReadmeMarkdown: row.ReadmeMarkdown,
		SourceKind:     externalSourceKindPrivateRegistry,
		SourceKey:      source.Key,
		SourceName:     source.Name,
		SourceTrust:    externalSourceTrustPrivate,
		ImportMode:     externalSourceKindPrivateRegistry,
		Tags:           row.Tags,
		Version:        row.Version,
		ArtifactSHA256: row.SHA256,
		ArtifactSize:   row.Size,
	}
}

func (s *Service) privateSkillSource(ctx context.Context, sourceID string) (externalSkillSource, error) {
	if s.skillStore == nil {
		return externalSkillSource{}, errors.New("skill source store not configured")
	}
	row, err := s.skillStore.GetSource(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(sourceID))
	if err != nil {
		return externalSkillSource{}, err
	}
	if row == nil || sourceManagedBy(*row) != externalSourceManagedByUser || row.Kind != externalSourceKindPrivateRegistry {
		return externalSkillSource{}, errors.New("skill source not found")
	}
	source := s.externalSkillSourceFromEntity(*row)
	if !source.Enabled {
		return externalSkillSource{}, errors.New("该私有来源已停用")
	}
	return source, nil
}

func normalizePrivateSourceAuthType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", externalSourceAuthNone:
		return externalSourceAuthNone, nil
	case externalSourceAuthBearer:
		return externalSourceAuthBearer, nil
	default:
		return "", errors.New("auth_type 仅支持 none 或 bearer")
	}
}

func privateRegistryRequest(ctx context.Context, source externalSkillSource, method string, requestURL string) (*http.Request, error) {
	if _, err := resolvePrivateRegistryURL(source.URL, requestURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return nil, err
	}
	if source.AuthType == externalSourceAuthBearer {
		if strings.TrimSpace(source.Credential) == "" {
			return nil, errors.New("私有来源缺少 Bearer Token")
		}
		request.Header.Set("Authorization", "Bearer "+source.Credential)
	}
	request.Header.Set("Accept", "application/json, application/zip")
	return request, nil
}

func privateRegistryEndpoint(baseURL string, query string, skillID string, limit int) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return "", errors.New("私有来源 URL 不正确")
	}
	if !strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), "/api/skills") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/skills"
	}
	values := parsed.Query()
	if strings.TrimSpace(query) != "" {
		values.Set("q", strings.TrimSpace(query))
	}
	if strings.TrimSpace(skillID) != "" {
		values.Set("id", strings.TrimSpace(skillID))
	}
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func resolvePrivateRegistryURL(baseURL string, rawURL string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Host == "" {
		return "", errors.New("私有来源 URL 不正确")
	}
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || strings.TrimSpace(rawURL) == "" {
		return "", errors.New("链接不正确")
	}
	resolveBase := *base
	if !strings.HasSuffix(resolveBase.Path, "/") {
		resolveBase.Path += "/"
	}
	target = resolveBase.ResolveReference(target)
	if !strings.EqualFold(base.Scheme, target.Scheme) || !strings.EqualFold(base.Host, target.Host) {
		return "", errors.New("链接必须与私有来源同源")
	}
	if target.User != nil || target.Fragment != "" {
		return "", errors.New("链接不能包含用户信息或片段")
	}
	return target.String(), nil
}

func (s *Service) validatePrivateRegistryBaseURL(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return "", errors.New("私有来源 URL 不正确")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", errors.New("私有来源 URL 不能包含用户信息、查询参数或片段")
	}
	hostname := strings.ToLower(parsed.Hostname())
	loopback := hostname == "localhost"
	if address := net.ParseIP(hostname); address != nil {
		loopback = address.IsLoopback()
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return "", errors.New("私有来源仅支持 HTTPS；本机回环地址可使用 HTTP")
	}
	if state, ok := authctx.StateFromContext(ctx); ok && state.AuthRequired {
		if parsed.Scheme != "https" {
			return "", errors.New("认证部署的私有来源必须使用 HTTPS")
		}
		if !privateSourceHostAllowed(parsed, s.config.SkillsPrivateSourceAllowedHosts) {
			return "", errors.New("私有来源主机未在 SKILLS_PRIVATE_SOURCE_ALLOWED_HOSTS 中")
		}
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func privateSourceHostAllowed(target *url.URL, allowed []string) bool {
	for _, raw := range allowed {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if parsed, err := url.Parse(candidate); err == nil && parsed.Host != "" {
			candidate = parsed.Host
		}
		if strings.EqualFold(candidate, target.Host) || strings.EqualFold(candidate, target.Hostname()) {
			return true
		}
	}
	return false
}
