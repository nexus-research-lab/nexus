package skills

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
)

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
