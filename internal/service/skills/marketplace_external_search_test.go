package skills

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchExternalSkillsAggregatesSources(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/api/search", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"skills": [
				{
					"id": "demo/community/skills-sh-demo",
					"skillId": "skills-sh-demo",
					"name": "skills-sh-demo",
					"source": "demo/community",
					"description": "from skills.sh source",
					"installs": 12,
					"tags": ["demo"]
				}
			]
		}`))
	})
	mux.HandleFunc("/agentskills.json", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"skills": [
				{
					"name": "git-demo",
					"title": "Git Demo",
					"description": "from index git source",
					"git_url": "https://github.com/example/skills",
					"git_branch": "main",
					"git_path": "skills/git-demo",
					"installs": 3,
					"tags": ["git", "demo"]
				},
				{
					"name": "url-demo",
					"description": "from index url source",
					"raw_url": "` + server.URL + `/url-demo/SKILL.md",
					"tags": "url,demo"
				}
			]
		}`))
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsAPIURL = server.URL
	cfg.SkillsAPISearchLimit = 10
	cfg.SkillsSourceURLs = "Test Hub|" + server.URL + "/agentskills.json"
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)

	result, err := service.SearchExternalSkills(context.Background(), "demo", false)
	if err != nil {
		t.Fatalf("搜索外部 skill 失败: %v", err)
	}
	sources, err := service.ListExternalSkillSources(context.Background())
	if err != nil {
		t.Fatalf("读取外部 skill 来源失败: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("默认来源未写入数据库: %+v", sources)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("来源状态数量不正确: %+v", result.Sources)
	}
	if len(result.Results) != 3 {
		t.Fatalf("聚合搜索结果数量不正确: %+v", result.Results)
	}
	gitItem := findExternalSearchItem(result.Results, "git-demo")
	if gitItem == nil || gitItem.ImportMode != externalSourceKindGit || gitItem.GitPath != "skills/git-demo" {
		t.Fatalf("Git 来源结果不正确: %+v", gitItem)
	}
	urlItem := findExternalSearchItem(result.Results, "url-demo")
	if urlItem == nil || urlItem.ImportMode != externalSourceKindURL || len(urlItem.Tags) != 2 {
		t.Fatalf("URL 来源结果不正确: %+v", urlItem)
	}
	skillsShItem := findExternalSearchItem(result.Results, "skills-sh-demo")
	if skillsShItem == nil || skillsShItem.ImportMode != externalSourceKindSkillsSh || skillsShItem.SourceName != "skills.sh" {
		t.Fatalf("skills.sh 来源结果不正确: %+v", skillsShItem)
	}
	if skillsShItem.DetailURL != server.URL+"/demo/community/skills-sh-demo" || skillsShItem.PackageSpec != "demo/community/skills-sh-demo" {
		t.Fatalf("skills.sh 详情链接不正确: %+v", skillsShItem)
	}
}

func TestDefaultExternalSkillSourcesIncludeCommunityRegistries(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	cfg.SkillsDefaultSourcesEnabled = true
	cfg.SkillsAPIURL = "https://skills.example"
	service := NewService(cfg, nil, nil)

	sources := service.configuredExternalSkillSources()
	expectedKinds := []string{
		externalSourceKindClaudePlugins,
		externalSourceKindSkillsSh,
		externalSourceKindClawhub,
		externalSourceKindBrowseSh,
		externalSourceKindHermesIndex,
	}
	if len(sources) != len(expectedKinds) {
		t.Fatalf("默认来源数量不正确: %+v", sources)
	}
	for index, kind := range expectedKinds {
		if sources[index].Kind != kind {
			t.Fatalf("默认来源顺序不正确: %+v", sources)
		}
	}
	if !sources[3].Enabled || sources[4].Enabled {
		t.Fatalf("默认来源开关状态不正确: %+v", sources)
	}
}

func TestSearchExternalSkillSourceSupportsCommunityRegistries(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/claude/api/skills", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("q") != "demo" || request.URL.Query().Get("limit") != "2" {
			t.Fatalf("claude-plugins 查询参数不正确: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"skills": [
				{
					"id": "uuid-demo",
					"name": "claude-demo",
					"namespace": "@demo/skills/claude-demo",
					"sourceUrl": "https://github.com/demo/skills/tree/main/skills/claude-demo",
					"description": "from claude plugins",
					"installs": 21,
					"metadata": {
						"repoOwner": "demo",
						"repoName": "skills",
						"directoryPath": "skills/claude-demo",
						"rawFileUrl": "https://raw.githubusercontent.com/demo/skills/main/skills/claude-demo/SKILL.md"
					}
				}
			]
		}`))
	})
	mux.HandleFunc("/claw/api/v1/search", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("q") != "demo" {
			t.Fatalf("clawhub 查询参数不正确: %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"results": [
				{
					"slug": "claw-demo",
					"displayName": "Claw Demo",
					"summary": "from clawhub",
					"version": "0.1.0",
					"ownerHandle": "owner-one",
					"downloads": 44
				}
			]
		}`))
	})
	mux.HandleFunc("/hermes-index.json", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"skills": [
				{
					"name": "hermes-demo",
					"description": "from hermes index",
					"source": "github",
					"identifier": "github/demo/skills/skills/hermes-demo",
					"trust_level": "community",
					"tags": ["demo"],
					"extra": {"installs": 8},
					"resolved_github_id": "demo/skills/skills/hermes-demo"
				}
			]
		}`))
	})
	mux.HandleFunc("/browse/api/skills", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"skills": [
				{
					"slug": "example.com/demo-task",
					"name": "browse-demo",
					"title": "Browse Demo",
					"description": "from browse.sh",
					"hostname": "example.com",
					"tags": ["demo"],
					"installCount": 9,
					"sourceUrl": "https://github.com/browserbase/browse.sh/blob/main/skills/example.com/demo-task/SKILL.md"
				}
			]
		}`))
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsAPISearchLimit = 2
	service := NewService(cfg, nil, nil)
	claudeSource := externalSkillSource{
		Key:       "claude-test",
		Name:      "claude-plugins.dev",
		Kind:      externalSourceKindClaudePlugins,
		URL:       server.URL + "/claude/api/skills",
		Trust:     externalSourceTrustCommunity,
		Enabled:   true,
		SortOrder: 0,
	}
	clawSource := externalSkillSource{
		Key:       "claw-test",
		Name:      "clawhub.ai",
		Kind:      externalSourceKindClawhub,
		URL:       server.URL + "/claw/api/v1/search",
		Trust:     externalSourceTrustCommunity,
		Enabled:   true,
		SortOrder: 10,
	}
	hermesSource := externalSkillSource{
		Key:       "hermes-test",
		Name:      "Hermes Skills Index",
		Kind:      externalSourceKindHermesIndex,
		URL:       server.URL + "/hermes-index.json",
		Trust:     externalSourceTrustCommunity,
		Enabled:   true,
		SortOrder: 20,
	}
	browseSource := externalSkillSource{
		Key:       "browse-test",
		Name:      "browse.sh",
		Kind:      externalSourceKindBrowseSh,
		URL:       server.URL + "/browse/api/skills",
		Trust:     externalSourceTrustCommunity,
		Enabled:   true,
		SortOrder: 30,
	}

	claudeItems, err := service.searchExternalSkillSource(context.Background(), claudeSource, "demo")
	if err != nil {
		t.Fatalf("claude-plugins 搜索失败: %v", err)
	}
	if len(claudeItems) != 1 || claudeItems[0].ImportMode != externalSourceKindGit || claudeItems[0].GitPath != "skills/claude-demo" {
		t.Fatalf("claude-plugins 结果不正确: %+v", claudeItems)
	}
	clawItems, err := service.searchExternalSkillSource(context.Background(), clawSource, "demo")
	if err != nil {
		t.Fatalf("clawhub 搜索失败: %v", err)
	}
	expectedDownloadURL := server.URL + "/claw/api/v1/download?slug=claw-demo"
	if len(clawItems) != 1 || clawItems[0].ImportMode != externalSourceKindURL || clawItems[0].RawURL != expectedDownloadURL {
		t.Fatalf("clawhub 结果不正确: %+v", clawItems)
	}
	if clawItems[0].DetailURL != server.URL+"/owner-one/claw-demo" || clawItems[0].Installs != 44 {
		t.Fatalf("clawhub 详情元数据不正确: %+v", clawItems[0])
	}
	hermesItems, err := service.searchExternalSkillSource(context.Background(), hermesSource, "demo")
	if err != nil {
		t.Fatalf("Hermes Index 搜索失败: %v", err)
	}
	if len(hermesItems) != 1 || hermesItems[0].ImportMode != externalSourceKindGit || hermesItems[0].GitPath != "skills/hermes-demo" {
		t.Fatalf("Hermes Index 结果不正确: %+v", hermesItems)
	}
	browseItems, err := service.searchExternalSkillSource(context.Background(), browseSource, "demo")
	if err != nil {
		t.Fatalf("browse.sh 搜索失败: %v", err)
	}
	expectedRawURL := "https://raw.githubusercontent.com/browserbase/browse.sh/main/skills/example.com/demo-task/SKILL.md"
	if len(browseItems) != 1 || browseItems[0].ImportMode != externalSourceKindURL || browseItems[0].RawURL != expectedRawURL {
		t.Fatalf("browse.sh 结果不正确: %+v", browseItems)
	}
}

func findExternalSearchItem(items []ExternalSkillSearchItem, name string) *ExternalSkillSearchItem {
	for index := range items {
		if items[index].Name == name {
			return &items[index]
		}
	}
	return nil
}
