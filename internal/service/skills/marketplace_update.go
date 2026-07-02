package skills

import (
	"context"
	"errors"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
	records, err := s.loadExternalRecords(ctx)
	if err != nil {
		return nil, err
	}
	result := &UpdateInstalledSkillsResponse{
		UpdatedSkills: make([]string, 0),
		SkippedSkills: make([]string, 0),
		Failures:      make([]SkillActionFailure, 0),
	}
	names := slices.Sorted(maps.Keys(records))
	for _, name := range names {
		detail, updateErr := s.updateSingleSkillRecord(ctx, records[name])
		if updateErr != nil {
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
		if redeployErr := s.redeploySkillToInstalledAgents(ctx, detail.Name); redeployErr != nil {
			result.Failures = append(result.Failures, SkillActionFailure{
				SkillName: name,
				Error:     redeployErr.Error(),
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
	detail, err := s.updateSingleSkillRecord(ctx, record)
	if err != nil {
		return nil, err
	}
	if err = s.redeploySkillToInstalledAgents(ctx, detail.Name); err != nil {
		return nil, err
	}
	return detail, nil
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

func (s *Service) checkSingleSkillRecord(ctx context.Context, record catalogRecord) (bool, error) {
	manifest, err := s.readManifest(record.SourcePath)
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
	default:
		return false, errSkillUpdateCheckUnsupported
	}
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
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || strings.HasPrefix(fields[0], "ref:") {
			continue
		}
		return fields[0], nil
	}
	return "", errors.New("未读取到远端 Git commit")
}

func (s *Service) checkURLSkillUpdate(ctx context.Context, record catalogRecord, manifest externalManifest) (bool, error) {
	currentHash := hashSkillContent(record.SourcePath)
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
	return s.skillStore.RecordImportedSkillCheck(ctx, authctx.OwnerUserID(ctx), skillName, updateAvailable, time.Now().UTC(), lastError)
}
