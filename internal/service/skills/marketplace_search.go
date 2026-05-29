package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
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
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Installs != items[j].Installs {
			return items[i].Installs > items[j].Installs
		}
		if items[i].SourceName != items[j].SourceName {
			return items[i].SourceName < items[j].SourceName
		}
		return items[i].Name < items[j].Name
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
		name := anyString(row["name"])
		if name == "" {
			continue
		}
		metadata := anyMap(row["metadata"])
		repoOwner := anyString(metadata["repoOwner"])
		repoName := anyString(metadata["repoName"])
		gitPath := anyString(metadata["directoryPath"])
		rawURL := anyString(metadata["rawFileUrl"])
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
		detailURL := firstNonEmpty(anyString(row["sourceUrl"]), githubTreeURL(gitURL, gitPath), rawURL, gitURL)
		items = append(items, ExternalSkillSearchItem{
			Name:           name,
			Title:          firstNonEmpty(anyString(row["title"]), name),
			Description:    firstNonEmpty(repairClaudePluginsText(anyString(row["description"])), "来自 claude-plugins.dev 的搜索结果"),
			Source:         firstNonEmpty(anyString(row["namespace"]), gitURL, source.URL),
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
			Version:        firstNonEmpty(anyString(row["version"]), packageSpec),
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
		id := anyString(row["id"])
		sourceRef := firstNonEmpty(anyString(row["source"]), skillsShSourceFromID(id))
		skillSlug := firstNonEmpty(anyString(row["skillId"]), anyString(row["skill_id"]), anyString(row["slug"]), skillsShSkillFromID(id), anyString(row["name"]))
		name := firstNonEmpty(anyString(row["name"]), skillSlug)
		if name == "" || skillSlug == "" {
			continue
		}
		packageSpec := buildSkillsPackageSpec(firstNonEmpty(id, sourceRef), skillSlug, name)
		item := ExternalSkillSearchItem{
			Name:           name,
			Title:          firstNonEmpty(anyString(row["title"]), name),
			Description:    firstNonEmpty(anyString(row["description"]), "来自 skills.sh 的搜索结果"),
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
			Version:        firstNonEmpty(anyString(row["version"]), packageSpec),
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
		slug := anyString(row["slug"])
		if slug == "" {
			continue
		}
		owner := firstNonEmpty(anyString(row["ownerHandle"]), anyString(anyMap(row["owner"])["handle"]))
		name := firstNonEmpty(anyString(row["displayName"]), anyString(row["display_name"]), slug)
		rawURL := clawhubDownloadURL(source.URL, slug)
		if rawURL == "" {
			continue
		}
		stats := anyMap(row["stats"])
		items = append(items, ExternalSkillSearchItem{
			Name:           slug,
			Title:          name,
			Description:    firstNonEmpty(anyString(row["summary"]), anyString(row["description"]), "来自 clawhub.ai 的搜索结果"),
			Source:         firstNonEmpty(owner, source.URL),
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
			Version:        firstNonEmpty(anyString(row["version"]), slug),
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

func externalIndexRows(payload any) []map[string]any {
	switch typed := payload.(type) {
	case []any:
		return anyMapRows(typed)
	case map[string]any:
		for _, key := range []string{"skills", "items", "results"} {
			if rows := anyMapRows(typed[key]); len(rows) > 0 {
				return rows
			}
		}
	}
	return []map[string]any{}
}

func anyMapRows(value any) []map[string]any {
	rawRows, ok := value.([]any)
	if !ok {
		return []map[string]any{}
	}
	rows := make([]map[string]any, 0, len(rawRows))
	for _, raw := range rawRows {
		row, ok := raw.(map[string]any)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func externalIndexRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	name := firstNonEmpty(anyString(row["name"]), anyString(row["id"]), anyString(row["slug"]))
	slug := firstNonEmpty(anyString(row["slug"]), name)
	gitURL := firstNonEmpty(anyString(row["git_url"]), anyString(row["repository_url"]), anyString(row["repo_url"]))
	gitBranch := firstNonEmpty(anyString(row["git_branch"]), anyString(row["branch"]), anyString(row["ref"]))
	gitPath := firstNonEmpty(anyString(row["git_path"]), anyString(row["skill_path"]), anyString(row["path"]))
	rawURL := firstNonEmpty(anyString(row["raw_url"]), anyString(row["skill_url"]), anyString(row["archive_url"]))
	if rawURL == "" && externalURLLooksImportable(anyString(row["url"])) {
		rawURL = anyString(row["url"])
	}
	packageSpec := firstNonEmpty(anyString(row["package_spec"]), gitURL, rawURL, anyString(row["source"]))
	detailURL := firstNonEmpty(anyString(row["detail_url"]), anyString(row["homepage"]), anyString(row["readme_url"]), rawURL, gitURL)
	importMode := normalizeImportMode(firstNonEmpty(
		anyString(row["import_mode"]),
		inferExternalImportMode(ExternalSkillSearchItem{GitURL: gitURL, RawURL: rawURL, PackageSpec: packageSpec}),
	))
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          firstNonEmpty(anyString(row["title"]), name),
		Description:    firstNonEmpty(anyString(row["description"]), anyString(row["summary"]), "来自外部来源的技能"),
		Source:         firstNonEmpty(anyString(row["source"]), source.URL),
		PackageSpec:    packageSpec,
		SkillSlug:      slug,
		Installs:       anyInt(row["installs"]),
		DetailURL:      detailURL,
		ReadmeMarkdown: anyString(row["readme_markdown"]),
		SourceKind:     source.Kind,
		SourceKey:      source.Key,
		SourceName:     source.Name,
		SourceTrust:    source.Trust,
		ImportMode:     importMode,
		GitURL:         gitURL,
		GitBranch:      gitBranch,
		GitPath:        gitPath,
		RawURL:         rawURL,
		Tags:           anyStringSlice(row["tags"]),
		Version:        firstNonEmpty(anyString(row["version"]), packageSpec),
	}
}

func hermesIndexRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	name := firstNonEmpty(anyString(row["name"]), anyString(row["id"]), anyString(row["identifier"]))
	identifier := anyString(row["identifier"])
	gitIdentifier := firstNonEmpty(anyString(row["resolved_github_id"]), githubIdentifierFromRepoPath(anyString(row["repo"]), anyString(row["path"])))
	gitURL, gitPath := splitGitHubIdentifier(gitIdentifier)
	extra := anyMap(row["extra"])
	detailURL := firstNonEmpty(anyString(extra["detail_url"]), githubTreeURL(gitURL, gitPath), anyString(extra["repo_url"]))
	sourceLabel := firstNonEmpty(anyString(row["source"]), source.Name)
	trust := normalizeExternalTrust(firstNonEmpty(anyString(row["trust_level"]), source.Trust))
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          firstNonEmpty(anyString(row["title"]), name),
		Description:    firstNonEmpty(anyString(row["description"]), "来自 Hermes Skills Index 的搜索结果"),
		Source:         identifier,
		PackageSpec:    gitURL,
		SkillSlug:      name,
		Installs:       anyInt(extra["installs"]),
		DetailURL:      detailURL,
		ReadmeMarkdown: "",
		SourceKind:     externalSourceKindHermesIndex,
		SourceKey:      source.Key,
		SourceName:     source.Name + " / " + sourceLabel,
		SourceTrust:    trust,
		ImportMode:     externalSourceKindGit,
		GitURL:         gitURL,
		GitPath:        gitPath,
		Tags:           anyStringSlice(row["tags"]),
		Version:        firstNonEmpty(anyString(row["generated_at"]), gitIdentifier),
	}
}

func browseShRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	slug := anyString(row["slug"])
	name := firstNonEmpty(anyString(row["name"]), anyString(row["task"]), slug)
	title := firstNonEmpty(anyString(row["title"]), name)
	rawURL := githubBlobToRawURL(anyString(row["sourceUrl"]))
	if rawURL == "" && externalURLLooksImportable(anyString(row["skillMdUrl"])) {
		rawURL = anyString(row["skillMdUrl"])
	}
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          title,
		Description:    firstNonEmpty(anyString(row["description"]), "来自 browse.sh 的网站自动化技能"),
		Source:         firstNonEmpty(anyString(row["hostname"]), anyString(row["source"]), source.URL),
		PackageSpec:    rawURL,
		SkillSlug:      firstNonEmpty(slug, name),
		Installs:       anyInt(row["installCount"]),
		DetailURL:      firstNonEmpty(rawURL, anyString(row["sourceUrl"])),
		ReadmeMarkdown: "",
		SourceKind:     externalSourceKindBrowseSh,
		SourceKey:      source.Key,
		SourceName:     source.Name,
		SourceTrust:    source.Trust,
		ImportMode:     externalSourceKindURL,
		RawURL:         rawURL,
		Tags:           anyStringSlice(row["tags"]),
		Version:        firstNonEmpty(anyString(row["updated"]), rawURL),
	}
}

func externalPointerSourceItem(source externalSkillSource) ExternalSkillSearchItem {
	name := skillNameFromSourceURL(source.URL)
	item := ExternalSkillSearchItem{
		Name:        name,
		Title:       name,
		Description: "来自 " + source.Name + " 的外部技能来源",
		Source:      source.URL,
		PackageSpec: source.URL,
		SkillSlug:   name,
		DetailURL:   source.URL,
		SourceKind:  source.Kind,
		SourceKey:   source.Key,
		SourceName:  source.Name,
		SourceTrust: source.Trust,
		ImportMode:  source.Kind,
		Version:     source.URL,
	}
	if source.Kind == externalSourceKindGit {
		item.GitURL = source.URL
	} else {
		item.RawURL = source.URL
	}
	return item
}

func externalItemMatchesQuery(item ExternalSkillSearchItem, needle string) bool {
	query := strings.ToLower(strings.TrimSpace(needle))
	if query == "" {
		return true
	}
	values := []string{item.Name, item.Title, item.Description, item.Source, item.PackageSpec, item.SourceName}
	values = append(values, item.Tags...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func dedupeExternalItems(items []ExternalSkillSearchItem) []ExternalSkillSearchItem {
	seen := map[string]struct{}{}
	result := make([]ExternalSkillSearchItem, 0, len(items))
	for _, item := range items {
		key := firstNonEmpty(item.SourceKey, item.PackageSpec, item.GitURL, item.RawURL, item.DetailURL) + "::" + firstNonEmpty(item.SkillSlug, item.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
