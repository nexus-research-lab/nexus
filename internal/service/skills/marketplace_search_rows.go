package skills

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
