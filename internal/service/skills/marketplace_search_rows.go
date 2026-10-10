package skills

import "github.com/nexus-research-lab/nexus/internal/infra/textutil"

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
	name := textutil.FirstNonEmpty(textutil.AnyString(row["name"]), textutil.AnyString(row["id"]), textutil.AnyString(row["slug"]))
	slug := textutil.FirstNonEmpty(textutil.AnyString(row["slug"]), name)
	gitURL := textutil.FirstNonEmpty(textutil.AnyString(row["git_url"]), textutil.AnyString(row["repository_url"]), textutil.AnyString(row["repo_url"]))
	gitBranch := textutil.FirstNonEmpty(textutil.AnyString(row["git_branch"]), textutil.AnyString(row["branch"]), textutil.AnyString(row["ref"]))
	gitPath := textutil.FirstNonEmpty(textutil.AnyString(row["git_path"]), textutil.AnyString(row["skill_path"]), textutil.AnyString(row["path"]))
	rawURL := textutil.FirstNonEmpty(textutil.AnyString(row["raw_url"]), textutil.AnyString(row["skill_url"]), textutil.AnyString(row["archive_url"]))
	if rawURL == "" && externalURLLooksImportable(textutil.AnyString(row["url"])) {
		rawURL = textutil.AnyString(row["url"])
	}
	packageSpec := textutil.FirstNonEmpty(textutil.AnyString(row["package_spec"]), gitURL, rawURL, textutil.AnyString(row["source"]))
	detailURL := textutil.FirstNonEmpty(textutil.AnyString(row["detail_url"]), textutil.AnyString(row["homepage"]), textutil.AnyString(row["readme_url"]), rawURL, gitURL)
	importMode := normalizeImportMode(textutil.FirstNonEmpty(
		textutil.AnyString(row["import_mode"]),
		inferExternalImportMode(ExternalSkillSearchItem{GitURL: gitURL, RawURL: rawURL, PackageSpec: packageSpec}),
	))
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          textutil.FirstNonEmpty(textutil.AnyString(row["title"]), name),
		Description:    textutil.FirstNonEmpty(textutil.AnyString(row["description"]), textutil.AnyString(row["summary"])),
		Source:         textutil.FirstNonEmpty(textutil.AnyString(row["source"]), source.URL),
		PackageSpec:    packageSpec,
		SkillSlug:      slug,
		Installs:       anyInt(row["installs"]),
		DetailURL:      detailURL,
		ReadmeMarkdown: textutil.AnyString(row["readme_markdown"]),
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
		Version:        textutil.FirstNonEmpty(textutil.AnyString(row["version"]), packageSpec),
	}
}

func hermesIndexRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	name := textutil.FirstNonEmpty(textutil.AnyString(row["name"]), textutil.AnyString(row["id"]), textutil.AnyString(row["identifier"]))
	identifier := textutil.AnyString(row["identifier"])
	gitIdentifier := textutil.FirstNonEmpty(textutil.AnyString(row["resolved_github_id"]), githubIdentifierFromRepoPath(textutil.AnyString(row["repo"]), textutil.AnyString(row["path"])))
	gitURL, gitPath := splitGitHubIdentifier(gitIdentifier)
	extra := anyMap(row["extra"])
	detailURL := textutil.FirstNonEmpty(textutil.AnyString(extra["detail_url"]), githubTreeURL(gitURL, gitPath), textutil.AnyString(extra["repo_url"]))
	sourceLabel := textutil.FirstNonEmpty(textutil.AnyString(row["source"]), source.Name)
	trust := normalizeExternalTrust(textutil.FirstNonEmpty(textutil.AnyString(row["trust_level"]), source.Trust))
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          textutil.FirstNonEmpty(textutil.AnyString(row["title"]), name),
		Description:    textutil.AnyString(row["description"]),
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
		Version:        textutil.FirstNonEmpty(textutil.AnyString(row["generated_at"]), gitIdentifier),
	}
}

func browseShRowItem(source externalSkillSource, row map[string]any) ExternalSkillSearchItem {
	slug := textutil.AnyString(row["slug"])
	name := textutil.FirstNonEmpty(textutil.AnyString(row["name"]), textutil.AnyString(row["task"]), slug)
	title := textutil.FirstNonEmpty(textutil.AnyString(row["title"]), name)
	rawURL := githubBlobToRawURL(textutil.AnyString(row["sourceUrl"]))
	if rawURL == "" && externalURLLooksImportable(textutil.AnyString(row["skillMdUrl"])) {
		rawURL = textutil.AnyString(row["skillMdUrl"])
	}
	return ExternalSkillSearchItem{
		Name:           name,
		Title:          title,
		Description:    textutil.AnyString(row["description"]),
		Source:         textutil.FirstNonEmpty(textutil.AnyString(row["hostname"]), textutil.AnyString(row["source"]), source.URL),
		PackageSpec:    rawURL,
		SkillSlug:      textutil.FirstNonEmpty(slug, name),
		Installs:       anyInt(row["installCount"]),
		DetailURL:      textutil.FirstNonEmpty(rawURL, textutil.AnyString(row["sourceUrl"])),
		ReadmeMarkdown: "",
		SourceKind:     externalSourceKindBrowseSh,
		SourceKey:      source.Key,
		SourceName:     source.Name,
		SourceTrust:    source.Trust,
		ImportMode:     externalSourceKindURL,
		RawURL:         rawURL,
		Tags:           anyStringSlice(row["tags"]),
		Version:        textutil.FirstNonEmpty(textutil.AnyString(row["updated"]), rawURL),
	}
}

func externalPointerSourceItem(source externalSkillSource) ExternalSkillSearchItem {
	name := skillNameFromSourceURL(source.URL)
	item := ExternalSkillSearchItem{
		Name:        name,
		Title:       name,
		Description: "",
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
