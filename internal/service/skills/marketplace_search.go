package skills

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

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
	slices.SortFunc(items, func(left ExternalSkillSearchItem, right ExternalSkillSearchItem) int {
		if result := cmp.Compare(right.Installs, left.Installs); result != 0 {
			return result
		}
		if result := cmp.Compare(left.SourceName, right.SourceName); result != 0 {
			return result
		}
		return cmp.Compare(left.Name, right.Name)
	})
	items = dedupeExternalItems(items)
	if includeReadme {
		s.attachExternalReadmes(ctx, items)
	}
	return &SearchExternalSkillsResponse{Query: needle, Results: items, Sources: statuses}, nil
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
