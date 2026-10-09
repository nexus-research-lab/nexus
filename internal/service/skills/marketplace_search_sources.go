package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
)

func (s *Service) searchClaudePluginsSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	requestURL, err := externalSearchURL(source.URL, "/api/skills", map[string]string{
		"q":     needle,
		"limit": fmt.Sprintf("%d", externalSkillSearchLimit(s.config.SkillsAPISearchLimit)),
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude-plugins.dev 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Skills []map[string]any `json:"skills"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("claude-plugins.dev 搜索返回 JSON 解析失败")
	}
	items := make([]ExternalSkillSearchItem, 0, len(payload.Skills))
	for _, row := range payload.Skills {
		name := textutil.AnyString(row["name"])
		if name == "" {
			continue
		}
		metadata := anyMap(row["metadata"])
		repoOwner := textutil.AnyString(metadata["repoOwner"])
		repoName := textutil.AnyString(metadata["repoName"])
		gitPath := textutil.AnyString(metadata["directoryPath"])
		rawURL := textutil.AnyString(metadata["rawFileUrl"])
		gitURL := ""
		if repoOwner != "" && repoName != "" {
			gitURL = "https://github.com/" + repoOwner + "/" + repoName
		}
		importMode := externalSourceKindGit
		packageSpec := gitURL
		if gitURL == "" && rawURL != "" {
			importMode = externalSourceKindURL
			packageSpec = rawURL
		}
		if packageSpec == "" {
			continue
		}
		detailURL := textutil.FirstNonEmpty(textutil.AnyString(row["sourceUrl"]), githubTreeURL(gitURL, gitPath), rawURL, gitURL)
		items = append(items, ExternalSkillSearchItem{
			Name:           name,
			Title:          textutil.FirstNonEmpty(textutil.AnyString(row["title"]), name),
			Description:    repairClaudePluginsText(textutil.AnyString(row["description"])),
			Source:         textutil.FirstNonEmpty(textutil.AnyString(row["namespace"]), gitURL, source.URL),
			PackageSpec:    packageSpec,
			SkillSlug:      name,
			Installs:       anyInt(row["installs"]),
			DetailURL:      detailURL,
			ReadmeMarkdown: "",
			SourceKind:     externalSourceKindClaudePlugins,
			SourceKey:      source.Key,
			SourceName:     source.Name,
			SourceTrust:    source.Trust,
			ImportMode:     importMode,
			GitURL:         gitURL,
			GitPath:        gitPath,
			RawURL:         rawURL,
			Tags:           anyStringSlice(row["tags"]),
			Version:        textutil.FirstNonEmpty(textutil.AnyString(row["version"]), packageSpec),
		})
	}
	return items, nil
}

func (s *Service) searchSkillsShSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	apiURL := strings.TrimRight(strings.TrimSpace(source.URL), "/")
	requestURL, err := externalSearchURL(apiURL, "/api/search", map[string]string{
		"q":     needle,
		"limit": fmt.Sprintf("%d", externalSkillSearchLimit(s.config.SkillsAPISearchLimit)),
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
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
		id := textutil.AnyString(row["id"])
		sourceRef := textutil.FirstNonEmpty(textutil.AnyString(row["source"]), skillsShSourceFromID(id))
		skillSlug := textutil.FirstNonEmpty(textutil.AnyString(row["skillId"]), textutil.AnyString(row["skill_id"]), textutil.AnyString(row["slug"]), skillsShSkillFromID(id), textutil.AnyString(row["name"]))
		name := textutil.FirstNonEmpty(textutil.AnyString(row["name"]), skillSlug)
		if name == "" || skillSlug == "" {
			continue
		}
		packageSpec := buildSkillsPackageSpec(textutil.FirstNonEmpty(id, sourceRef), skillSlug, name)
		item := ExternalSkillSearchItem{
			Name:           name,
			Title:          textutil.FirstNonEmpty(textutil.AnyString(row["title"]), name),
			Description:    textutil.AnyString(row["description"]),
			Source:         sourceRef,
			PackageSpec:    packageSpec,
			SkillSlug:      skillSlug,
			Installs:       anyInt(row["installs"]),
			DetailURL:      skillsShDetailURL(apiURL, sourceRef, skillSlug),
			ReadmeMarkdown: "",
			SourceKind:     externalSourceKindSkillsSh,
			SourceKey:      source.Key,
			SourceName:     source.Name,
			SourceTrust:    source.Trust,
			ImportMode:     externalSourceKindSkillsSh,
			Tags:           anyStringSlice(row["tags"]),
			Version:        textutil.FirstNonEmpty(textutil.AnyString(row["version"]), packageSpec),
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) searchClawhubSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	requestURL, err := externalSearchURL(source.URL, "/api/v1/search", map[string]string{"q": needle})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clawhub.ai 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Results []map[string]any `json:"results"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("clawhub.ai 搜索返回 JSON 解析失败")
	}
	items := make([]ExternalSkillSearchItem, 0, len(payload.Results))
	for _, row := range payload.Results {
		slug := textutil.AnyString(row["slug"])
		if slug == "" {
			continue
		}
		owner := textutil.FirstNonEmpty(textutil.AnyString(row["ownerHandle"]), textutil.AnyString(anyMap(row["owner"])["handle"]))
		name := textutil.FirstNonEmpty(textutil.AnyString(row["displayName"]), textutil.AnyString(row["display_name"]), slug)
		rawURL := clawhubDownloadURL(source.URL, slug)
		if rawURL == "" {
			continue
		}
		stats := anyMap(row["stats"])
		items = append(items, ExternalSkillSearchItem{
			Name:           slug,
			Title:          name,
			Description:    textutil.FirstNonEmpty(textutil.AnyString(row["summary"]), textutil.AnyString(row["description"])),
			Source:         textutil.FirstNonEmpty(owner, source.URL),
			PackageSpec:    rawURL,
			SkillSlug:      slug,
			Installs:       firstNonZero(anyInt(row["downloads"]), anyInt(row["installs"]), anyInt(row["installsAllTime"]), anyInt(stats["downloads"]), anyInt(stats["installsAllTime"])),
			DetailURL:      clawhubDetailURL(source.URL, owner, slug),
			ReadmeMarkdown: "",
			SourceKind:     externalSourceKindClawhub,
			SourceKey:      source.Key,
			SourceName:     source.Name,
			SourceTrust:    source.Trust,
			ImportMode:     externalSourceKindURL,
			RawURL:         rawURL,
			Version:        textutil.FirstNonEmpty(textutil.AnyString(row["version"]), slug),
		})
	}
	return items, nil
}

func (s *Service) searchHermesIndexSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Hermes Skills Index 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Skills []map[string]any `json:"skills"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("Hermes Skills Index 返回 JSON 解析失败")
	}
	limit := externalSkillSearchLimit(s.config.SkillsAPISearchLimit)
	items := make([]ExternalSkillSearchItem, 0, limit)
	for _, row := range payload.Skills {
		if len(items) >= limit {
			break
		}
		item := hermesIndexRowItem(source, row)
		if item.Name == "" || item.SkillSlug == "" || item.GitURL == "" || item.GitPath == "" {
			continue
		}
		if !externalItemMatchesQuery(item, needle) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) searchBrowseShSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("browse.sh 搜索失败: HTTP %d", response.StatusCode)
	}
	var payload struct {
		Skills []map[string]any `json:"skills"`
	}
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, errors.New("browse.sh 返回 JSON 解析失败")
	}
	limit := externalSkillSearchLimit(s.config.SkillsAPISearchLimit)
	items := make([]ExternalSkillSearchItem, 0, limit)
	for _, row := range payload.Skills {
		if len(items) >= limit {
			break
		}
		item := browseShRowItem(source, row)
		if item.Name == "" || item.SkillSlug == "" || item.RawURL == "" {
			continue
		}
		if !externalItemMatchesQuery(item, needle) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) searchWellKnownSource(ctx context.Context, source externalSkillSource, needle string) ([]ExternalSkillSearchItem, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := externalSkillsHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s 搜索失败: HTTP %d", source.Name, response.StatusCode)
	}
	var payload any
	if err = json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s 返回 JSON 解析失败", source.Name)
	}
	rows := externalIndexRows(payload)
	items := make([]ExternalSkillSearchItem, 0, len(rows))
	for _, row := range rows {
		item := externalIndexRowItem(source, row)
		if item.SkillSlug == "" || item.Name == "" {
			continue
		}
		if !externalItemMatchesQuery(item, needle) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}
