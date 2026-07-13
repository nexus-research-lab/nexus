package room

import (
	"context"
	"errors"
	"fmt"
	"strings"

	skillspkg "github.com/nexus-research-lab/nexus/internal/service/skills"
)

// RoomSkillCatalog 描述 Room 服务读取 skill 目录所需的最小接口。
type RoomSkillCatalog interface {
	GetSkillDetail(ctx context.Context, skillName string, agentID string) (*skillspkg.Detail, error)
}

// SetSkillCatalog 注入 Room Skill 目录解析器。
func (s *Service) SetSkillCatalog(catalog RoomSkillCatalog) {
	s.skills = catalog
}

func (s *Service) normalizeRoomSkillNames(ctx context.Context, values []string) ([]string, error) {
	details, err := s.resolveRoomSkillDetails(ctx, values)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(details))
	for _, detail := range details {
		result = append(result, detail.Name)
	}
	return result, nil
}

func (s *Service) resolveRoomSkillDetails(ctx context.Context, values []string) ([]skillspkg.Detail, error) {
	seen := map[string]struct{}{}
	result := make([]skillspkg.Detail, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if s.skills == nil {
			return nil, errors.New("room skill catalog 未配置")
		}
		detail, err := s.skills.GetSkillDetail(ctx, name, "")
		if err != nil {
			return nil, fmt.Errorf("room skill 不存在: %s", name)
		}
		if detail.Scope != skillspkg.ScopeRoom {
			return nil, fmt.Errorf("skill 不是 room scope: %s", name)
		}
		result = append(result, *detail)
	}
	return result, nil
}

// BuildRoomSkillPrompt 构造注入给 Room 成员的房间级规则。
func (s *Service) BuildRoomSkillPrompt(ctx context.Context, skillNames []string) (string, error) {
	details, err := s.resolveRoomSkillDetails(ctx, skillNames)
	if err != nil {
		return "", err
	}
	if len(details) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString("# Room Skills\n\n")
	builder.WriteString("These rules come from Room Skills enabled for the current Room and apply to every Room member.")
	for _, detail := range details {
		builder.WriteString("\n\n## ")
		builder.WriteString(detail.Title)
		builder.WriteString(" (`")
		builder.WriteString(detail.Name)
		builder.WriteString("`)\n\n")
		builder.WriteString(strings.TrimSpace(stripSkillFrontmatter(detail.ReadmeMarkdown)))
	}
	return builder.String(), nil
}

func stripSkillFrontmatter(content string) string {
	normalized := strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(normalized, "---") {
		return normalized
	}
	rest := strings.TrimPrefix(normalized, "---")
	_, body, ok := strings.Cut(rest, "\n---")
	if !ok {
		return normalized
	}
	return strings.TrimLeft(body, "\r\n")
}
