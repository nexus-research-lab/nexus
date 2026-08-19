// INPUT: 已持久化远端来源元数据、可选 expected catalog version 与更新检查请求。
// OUTPUT: 单 Skill 版本化原子更新、批量兼容结果及串行健康元数据写入。
// POS: 外部 Skill 更新检查与重新发布的业务边界。
package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	skillstore "github.com/nexus-research-lab/nexus/internal/storage/skills"
)

var errSkillUpdateCheckUnsupported = errors.New("该 skill 来源不支持检查更新")

// CheckImportedSkillUpdates 检查所有已导入外部技能是否有远端更新。
func (s *Service) CheckImportedSkillUpdates(ctx context.Context) (*CheckSkillUpdatesResponse, error) {
	records, err := s.loadExternalRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := &CheckSkillUpdatesResponse{
		AvailableSkills: make([]string, 0),
		SkippedSkills:   make([]string, 0),
		Failures:        make([]SkillActionFailure, 0),
	}
	names := slices.Sorted(maps.Keys(records))
	for _, name := range names {
		available, checkErr := s.checkSingleSkillRecord(ctx, records[name])
		switch {
		case errors.Is(checkErr, errSkillUpdateCheckUnsupported):
			result.SkippedSkills = append(result.SkippedSkills, name)
			if err := s.recordImportedSkillCheck(ctx, name, false, ""); err != nil {
				result.Failures = append(result.Failures, SkillActionFailure{SkillName: name, Error: err.Error()})
			}
		case checkErr != nil:
			result.Failures = append(result.Failures, SkillActionFailure{
				SkillName: name,
				Error:     checkErr.Error(),
			})
			if err := s.recordImportedSkillCheck(ctx, name, false, checkErr.Error()); err != nil {
				result.Failures = append(result.Failures, SkillActionFailure{SkillName: name, Error: err.Error()})
			}
		default:
			if available {
				result.AvailableSkills = append(result.AvailableSkills, name)
			}
			if err := s.recordImportedSkillCheck(ctx, name, available, ""); err != nil {
				result.Failures = append(result.Failures, SkillActionFailure{SkillName: name, Error: err.Error()})
			}
		}
	}
	return result, nil
}

// UpdateImportedSkills 更新所有已导入的外部技能。
func (s *Service) UpdateImportedSkills(ctx context.Context) (*UpdateInstalledSkillsResponse, error) {
	return s.updateImportedSkills(ctx, nil)
}

// UpdateImportedSkillsAtVersion 从一个 owner catalog version 开始串行更新全部远端 Skill。
//
// 每个成功发布的 Skill 都会产生一个新 catalog version；任意并发写一旦插入批次，
// 后续项会停止并返回 typed reconcile，避免把部分完成误报为普通成功。
func (s *Service) UpdateImportedSkillsAtVersion(
	ctx context.Context,
	expectedVersion int64,
) (*UpdateInstalledSkillsResponse, error) {
	state, err := s.GetCatalogState(ctx)
	if err != nil {
		return nil, err
	}
	if state.Version != expectedVersion {
		return nil, &skillstore.CatalogVersionConflictError{
			Expected: expectedVersion,
			Current:  state.Version,
		}
	}
	return s.updateImportedSkills(ctx, &expectedVersion)
}

func (s *Service) updateImportedSkills(
	ctx context.Context,
	expectedVersion *int64,
) (*UpdateInstalledSkillsResponse, error) {
	records, err := s.loadExternalRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := &UpdateInstalledSkillsResponse{
		UpdatedSkills: make([]string, 0),
		SkippedSkills: make([]string, 0),
		Failures:      make([]SkillActionFailure, 0),
		DeployResults: make([]SkillRedeployResult, 0),
	}
	names := slices.Sorted(maps.Keys(records))
	for _, name := range names {
		previousVersion := int64(0)
		if expectedVersion != nil {
			previousVersion = *expectedVersion
		}
		detail, updateErr := s.updateSingleSkillRecord(ctx, records[name], expectedVersion)
		if updateErr != nil {
			if errors.Is(updateErr, ErrCatalogVersionConflict) ||
				SkillMutationNeedsReconcile(updateErr) {
				if len(result.UpdatedSkills) > 0 &&
					!SkillMutationApplied(updateErr) {
					updateErr = &CatalogReconcileError{
						applied: true,
						cause:   updateErr,
					}
				}
				return result, updateErr
			}
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
		if expectedVersion != nil {
			state, stateErr := s.GetCatalogState(ctx)
			if stateErr != nil {
				return result, &CatalogReconcileError{
					applied: true,
					cause:   stateErr,
				}
			}
			if state.Version != previousVersion+1 {
				return result, &CatalogReconcileError{
					applied: true,
					cause: fmt.Errorf(
						"批量更新期间 catalog version 非预期变化: expected=%d actual=%d",
						previousVersion+1,
						state.Version,
					),
				}
			}
			*expectedVersion = state.Version
		}
		affectedAgents, listErr := s.agentsReferencingSkill(ctx, detail.Name)
		if listErr != nil {
			result.Failures = append(result.Failures, SkillActionFailure{
				SkillName: name,
				Error:     listErr.Error(),
			})
			continue
		}
		if len(affectedAgents) > 0 {
			result.DeployResults = append(result.DeployResults, SkillRedeployResult{
				SkillName:     name,
				SuccessAgents: affectedAgents,
				Failures:      []RedeployAgentFailure{},
			})
		}
	}
	return result, nil
}

// UpdateSingleSkill 更新单个已导入技能。
func (s *Service) UpdateSingleSkill(ctx context.Context, skillName string) (*Detail, error) {
	return s.updateSingleSkill(ctx, skillName, nil)
}

// UpdateSingleSkillAtVersion 仅在 owner catalog version 匹配时更新一个远端 Skill。
func (s *Service) UpdateSingleSkillAtVersion(
	ctx context.Context,
	skillName string,
	expectedVersion int64,
) (*Detail, error) {
	return s.updateSingleSkill(ctx, skillName, &expectedVersion)
}

func (s *Service) updateSingleSkill(
	ctx context.Context,
	skillName string,
	expectedVersion *int64,
) (*Detail, error) {
	records, err := s.loadExternalRecords(ctx)
	if err != nil {
		return nil, err
	}
	record, ok := records[strings.TrimSpace(skillName)]
	if !ok {
		return nil, errors.New("skill not found")
	}
	detail, err := s.updateSingleSkillRecord(ctx, record, expectedVersion)
	if err != nil {
		return nil, err
	}
	affectedAgents, err := s.agentsReferencingSkill(ctx, detail.Name)
	if err != nil {
		return nil, &CatalogReconcileError{applied: true, cause: err}
	}
	detail.DeploySuccesses = affectedAgents
	return detail, nil
}

func (s *Service) updateSingleSkillRecord(
	ctx context.Context,
	record catalogRecord,
	expectedVersion *int64,
) (*Detail, error) {
	manifest, err := s.manifestForRecord(ctx, record)
	if err != nil {
		return nil, err
	}
	switch manifest.ImportMode {
	case "git":
		return s.importGit(
			ctx,
			manifest.GitURL,
			manifest.GitBranch,
			manifest.GitPath,
			manifest,
			expectedVersion,
		)
	case "skills_sh":
		return s.importSkillsSh(ctx, manifest.SourceRef, manifest.Name, expectedVersion)
	case "url":
		return s.importSkillURL(
			ctx,
			firstNonEmpty(manifest.RawURL, manifest.SourceRef, manifest.DetailURL),
			manifest,
			expectedVersion,
		)
	case externalSourceKindPrivateRegistry:
		source, sourceErr := s.privateSkillSource(ctx, manifest.SourceKey)
		if sourceErr != nil {
			return nil, sourceErr
		}
		return s.importPrivateRegistrySkill(
			ctx,
			source,
			firstNonEmpty(manifest.SourceSkillID, manifest.SourceRef),
			manifest.Name,
			expectedVersion,
		)
	default:
		return nil, errors.New("该 skill 来源不支持更新")
	}
}

func (s *Service) checkSingleSkillRecord(ctx context.Context, record catalogRecord) (bool, error) {
	manifest, err := s.manifestForRecord(ctx, record)
	if err != nil {
		return false, err
	}
	switch manifest.ImportMode {
	case "git", "skills_sh":
		remoteCommit, err := s.remoteGitCommit(ctx, manifest)
		if err != nil {
			return false, err
		}
		return remoteCommit != "" && remoteCommit != strings.TrimSpace(manifest.GitCommit), nil
	case "url":
		return s.checkURLSkillUpdate(ctx, record, manifest)
	case externalSourceKindPrivateRegistry:
		return s.checkPrivateRegistrySkillUpdate(ctx, manifest)
	default:
		return false, errSkillUpdateCheckUnsupported
	}
}

func (s *Service) manifestForRecord(
	ctx context.Context,
	record catalogRecord,
) (externalManifest, error) {
	if strings.TrimSpace(record.Manifest.ImportMode) != "" {
		return record.Manifest, nil
	}
	if record.Detail.SourceType == sourceTypeExternal {
		ownerRoot, err := s.openOwnerSkillLibrary(ctx, false)
		if err != nil {
			return externalManifest{}, err
		}
		defer ownerRoot.Close()
		return readSkillManifestAtOwnerPath(ownerRoot, record.SourcePath)
	}
	return s.readManifest(record.SourcePath)
}

func (s *Service) hashSkillContentForRecord(
	ctx context.Context,
	record catalogRecord,
) string {
	if record.Detail.SourceType != sourceTypeExternal {
		return hashSkillContent(record.SourcePath)
	}
	ownerRoot, err := s.openOwnerSkillLibrary(ctx, false)
	if err != nil {
		return ""
	}
	defer ownerRoot.Close()
	payload, err := readSkillFileAtOwnerPath(ownerRoot, record.SourcePath, "SKILL.md")
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (s *Service) remoteGitCommit(ctx context.Context, manifest externalManifest) (string, error) {
	repositoryURL := strings.TrimSpace(manifest.GitURL)
	if repositoryURL == "" {
		return "", errors.New("缺少 Git 仓库地址")
	}
	options := gitCloneOptions{
		Branch:            strings.TrimSpace(manifest.GitBranch),
		CleanGlobalConfig: shouldUseCleanGitConfigForRepository(repositoryURL, manifest),
	}
	var output string
	var err error
	if options.Branch != "" {
		output, err = s.runCommandWithEnv(ctx, "", gitCommandEnv(options), "git", "ls-remote", "--heads", "--", repositoryURL, options.Branch)
	} else {
		output, err = s.runCommandWithEnv(ctx, "", gitCommandEnv(options), "git", "ls-remote", "--symref", "--", repositoryURL, "HEAD")
	}
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "ref:") {
			continue
		}
		return fields[0], nil
	}
	if branch := strings.TrimSpace(manifest.GitBranch); branch != "" {
		return "", fmt.Errorf(
			"此 Skill 记录的远端分支已不存在（%s），因此无法检查更新；请删除该 Skill 后从有效分支重新导入",
			branch,
		)
	}
	return "", errors.New("未读取到远端 Git commit")
}

func (s *Service) checkURLSkillUpdate(ctx context.Context, record catalogRecord, manifest externalManifest) (bool, error) {
	currentHash := s.hashSkillContentForRecord(ctx, record)
	if currentHash == "" {
		return false, errors.New("当前 skill 内容缺少 hash")
	}
	sourceURL := firstNonEmpty(manifest.RawURL, manifest.SourceRef, manifest.DetailURL)
	targetURL, err := s.validateExternalURL(ctx, sourceURL)
	if err != nil {
		return false, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return false, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, errors.New("skill URL 检查失败: HTTP " + response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalImportBytes+1))
	if err != nil {
		return false, err
	}
	if len(body) > maxExternalImportBytes {
		return false, errors.New("skill URL 内容超过大小限制")
	}
	tempDir, err := os.MkdirTemp("", "nexus-skill-check-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tempDir)
	if isZipPayload(targetURL, response.Header.Get("Content-Type"), body) {
		if err = unzipArchive(body, tempDir); err != nil {
			return false, err
		}
	} else if err = os.WriteFile(filepath.Join(tempDir, "SKILL.md"), body, 0o644); err != nil {
		return false, err
	}
	sourceDir, err := findSkillSourceDir(tempDir)
	if err != nil {
		return false, err
	}
	return hashSkillContent(sourceDir) != currentHash, nil
}

func (s *Service) recordImportedSkillCheck(ctx context.Context, skillName string, updateAvailable bool, lastError string) error {
	if s.skillStore == nil {
		return nil
	}
	_, err := s.withCatalogMutation(
		ctx,
		nil,
		false,
		func(mutation *skillstore.CatalogMutation) error {
			return mutation.RecordImportedSkillCheck(
				ctx,
				skillName,
				updateAvailable,
				time.Now().UTC(),
				lastError,
			)
		},
	)
	return err
}
