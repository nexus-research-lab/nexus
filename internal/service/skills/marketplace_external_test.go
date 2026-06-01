package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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

func TestImportSkillURLPersistsExternalManifest(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/url-demo/SKILL.md", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`---
name: url-demo
title: URL Demo
description: URL source demo
tags: [url]
---

# URL Demo
`))
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsAPIURL = ""
	cfg.SkillsSourceURLs = "URL Test|" + server.URL + "/url-demo/SKILL.md"
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)

	detail, err := service.ImportSkillURL(context.Background(), server.URL+"/url-demo/SKILL.md", externalManifest{
		SourceKind:  externalSourceKindURL,
		SourceKey:   "test-url",
		SourceName:  "URL Test",
		SourceTrust: externalSourceTrustCommunity,
	})
	if err != nil {
		t.Fatalf("URL 导入失败: %v", err)
	}
	if detail.Name != "url-demo" || !detail.HasUpdate {
		t.Fatalf("URL 导入详情不正确: %+v", detail)
	}
	manifest, err := service.readManifest(filepath.Join(service.registryRoot(context.Background()), "url-demo"))
	if err != nil {
		t.Fatalf("读取导入 manifest 失败: %v", err)
	}
	if manifest.ImportMode != externalSourceKindURL || manifest.RawURL == "" || manifest.SourceName != "URL Test" {
		t.Fatalf("导入 manifest 未记录来源: %+v", manifest)
	}
}

func TestImportSkillURLUsesManifestNameWhenFrontmatterOmitsName(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/metadata-name/SKILL.md", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`---
title: Metadata Named Skill
description: no name in frontmatter
---

# Metadata Named Skill
`))
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsAPIURL = ""
	cfg.SkillsSourceURLs = "URL Test|" + server.URL + "/metadata-name/SKILL.md"
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)

	detail, err := service.ImportSkillURL(context.Background(), server.URL+"/metadata-name/SKILL.md", externalManifest{
		Name:       "registry-demo",
		SourceKind: externalSourceKindURL,
		SourceName: "URL Test",
	})
	if err != nil {
		t.Fatalf("URL 导入失败: %v", err)
	}
	if detail.Name != "registry-demo" || strings.HasPrefix(detail.Name, "nexus-skill-url-") {
		t.Fatalf("URL 导入不应使用临时目录名: %+v", detail)
	}
}

func TestImportSkillURLInfersNameFromSkillMDParent(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/skills/example.com/demo-task/SKILL.md", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`# Demo Task
`))
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsAPIURL = ""
	cfg.SkillsSourceURLs = "URL Test|" + server.URL + "/skills/example.com/demo-task/SKILL.md"
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)

	detail, err := service.ImportSkillURL(context.Background(), server.URL+"/skills/example.com/demo-task/SKILL.md", externalManifest{})
	if err != nil {
		t.Fatalf("URL 导入失败: %v", err)
	}
	if detail.Name != "demo-task" {
		t.Fatalf("URL 导入未从 SKILL.md 父目录推断名字: %+v", detail)
	}
}

func TestPreviewAndImportSkillURLSupportZipPayloadWithoutZipSuffix(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	archive := buildTestSkillZip(t, "claw-zip-demo", "Claw Zip Demo")
	mux.HandleFunc("/api/v1/download", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(archive)
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsDefaultSourcesEnabled = false
	cfg.SkillsSourceURLs = "Claw Test|" + server.URL + "/api/v1/search"
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)
	downloadURL := server.URL + "/api/v1/download?slug=claw-zip-demo"

	preview, err := service.GetExternalSkillPreview(context.Background(), downloadURL)
	if err != nil {
		t.Fatalf("zip 预览失败: %v", err)
	}
	if !strings.Contains(preview.ReadmeMarkdown, "# Claw Zip Demo") || strings.Contains(preview.ReadmeMarkdown, "<html") {
		t.Fatalf("zip 预览内容不正确: %+v", preview)
	}

	detail, err := service.ImportSkillURL(context.Background(), downloadURL, externalManifest{
		SourceKind:  externalSourceKindClawhub,
		SourceName:  "clawhub.ai",
		SourceTrust: externalSourceTrustCommunity,
	})
	if err != nil {
		t.Fatalf("zip URL 导入失败: %v", err)
	}
	if detail.Name != "claw-zip-demo" || detail.Title != "Claw Zip Demo" {
		t.Fatalf("zip URL 导入详情不正确: %+v", detail.Info)
	}
}

func TestSkillsShSearchBuildsPreviewURLFromSourceAndSkillID(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/api/search", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"skills": [
				{
					"id": "membranedev/application-skills/pdfco",
					"skillId": "pdfco",
					"name": "pdfco",
					"installs": 101,
					"source": "membranedev/application-skills"
				}
			]
		}`))
	})

	service := NewService(newSkillsTestConfig(t), nil, nil)
	items, err := service.searchSkillsShSource(context.Background(), externalSkillSource{
		Name:    "skills.sh",
		Kind:    externalSourceKindSkillsSh,
		URL:     server.URL,
		Enabled: true,
	}, "pdf")
	if err != nil {
		t.Fatalf("skills.sh 搜索失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("skills.sh 搜索结果数量不正确: %+v", items)
	}
	if items[0].DetailURL != server.URL+"/membranedev/application-skills/pdfco" {
		t.Fatalf("skills.sh 预览 URL 不正确: %+v", items[0])
	}
	if items[0].PackageSpec != "membranedev/application-skills/pdfco" || items[0].SkillSlug != "pdfco" {
		t.Fatalf("skills.sh 导入元数据不正确: %+v", items[0])
	}
}

func TestImportSkillsShClonesRepositoryAndSelectsRequestedSkill(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)
	ctx := context.Background()

	repoRoot := filepath.Join(t.TempDir(), "repo")
	writeTestSkillDir(t, filepath.Join(repoRoot, "skills", "alpha"), "alpha", "Alpha Skill", false)
	writeTestSkillDir(t, filepath.Join(repoRoot, "skills", "pdfco"), "pdfco", "PDF Skill", false)
	service.commandRunner = func(_ context.Context, workDir string, extraEnv []string, command ...string) (string, error) {
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "ls-remote") {
			if !stringSliceContainsPrefix(extraEnv, "GIT_CONFIG_GLOBAL=") {
				t.Fatalf("skills.sh Git 分支探测应隔离全局 Git 配置: %+v", extraEnv)
			}
			return "ref: refs/heads/main\tHEAD\n", nil
		}
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "clone") {
			if !stringSliceContainsPrefix(extraEnv, "GIT_CONFIG_GLOBAL=") {
				t.Fatalf("skills.sh Git 导入应隔离全局 Git 配置: %+v", extraEnv)
			}
			if got, want := command[len(command)-2], "https://github.com/membranedev/application-skills"; got != want {
				t.Fatalf("skills.sh Git 仓库不正确: got=%q want=%q", got, want)
			}
			if stringSliceContains(command, "--sparse") || stringSliceContains(command, "--filter=blob:none") {
				t.Fatalf("skills.sh Git 导入不应使用 partial/sparse clone: %+v", command)
			}
			if !stringSliceContains(command, "--branch") || !stringSliceContains(command, "main") {
				t.Fatalf("skills.sh Git 导入应解析并使用默认分支: %+v", command)
			}
			if !stringSliceContains(command, "--") {
				t.Fatalf("skills.sh Git 导入应使用 -- 分隔仓库参数: %+v", command)
			}
			return "", copyDirectory(repoRoot, command[len(command)-1])
		}
		if len(command) >= 3 && command[0] == "git" && command[1] == "rev-parse" && workDir != "" {
			return "commit-skills-sh", nil
		}
		return "", errors.New("unexpected command")
	}

	detail, err := service.ImportSkillsSh(ctx, "membranedev/application-skills/pdfco", "pdfco")
	if err != nil {
		t.Fatalf("skills.sh Git 导入失败: %v", err)
	}
	if detail.Name != "pdfco" || detail.Title != "PDF Skill" {
		t.Fatalf("skills.sh 导入未选中指定 skill: %+v", detail.Info)
	}
	if detail.SourceKind != externalSourceKindSkillsSh || detail.ImportMode != externalSourceKindSkillsSh || detail.Version != "commit-skills-sh" {
		t.Fatalf("skills.sh 导入元数据不正确: %+v", detail.Info)
	}
	record, err := service.skillStore.GetImportedSkill(ctx, authctx.OwnerUserID(ctx), "pdfco")
	if err != nil {
		t.Fatalf("读取 skills.sh 导入记录失败: %v", err)
	}
	if record == nil || record.GitURL != "https://github.com/membranedev/application-skills" || record.GitPath != "skills/pdfco" || record.SourceRef != "membranedev/application-skills/pdfco" {
		t.Fatalf("skills.sh 导入 DB 记录不正确: %+v", record)
	}
}

func TestImportSkillsShRetriesTransientGitCloneEOF(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)
	ctx := context.Background()

	repoRoot := filepath.Join(t.TempDir(), "repo")
	writeTestSkillDir(t, filepath.Join(repoRoot, "skills", "pdfco"), "pdfco", "PDF Skill", false)
	cloneAttempts := 0
	service.commandRunner = func(_ context.Context, workDir string, extraEnv []string, command ...string) (string, error) {
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "ls-remote") {
			if !stringSliceContainsPrefix(extraEnv, "GIT_CONFIG_GLOBAL=") {
				t.Fatalf("skills.sh Git 分支探测应隔离全局 Git 配置: %+v", extraEnv)
			}
			return "ref: refs/heads/main\tHEAD\n", nil
		}
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "clone") {
			if !stringSliceContainsPrefix(extraEnv, "GIT_CONFIG_GLOBAL=") {
				t.Fatalf("skills.sh Git 导入应隔离全局 Git 配置: %+v", extraEnv)
			}
			cloneAttempts++
			if cloneAttempts == 1 {
				return "fatal: early EOF", errors.New("exit status 128")
			}
			return "", copyDirectory(repoRoot, command[len(command)-1])
		}
		if len(command) >= 3 && command[0] == "git" && command[1] == "rev-parse" && workDir != "" {
			return "commit-skills-sh", nil
		}
		return "", errors.New("unexpected command")
	}

	detail, err := service.ImportSkillsSh(ctx, "membranedev/application-skills/pdfco", "pdfco")
	if err != nil {
		t.Fatalf("skills.sh Git 导入重试后仍失败: %v", err)
	}
	if cloneAttempts != 2 {
		t.Fatalf("skills.sh Git 导入未按 transient EOF 重试: %d", cloneAttempts)
	}
	if detail.Name != "pdfco" || detail.Version != "commit-skills-sh" {
		t.Fatalf("skills.sh 重试后导入结果不正确: %+v", detail.Info)
	}
}

func TestGitCloneTransientErrorDetectionCoversSSLDrops(t *testing.T) {
	cases := []struct {
		name   string
		output string
		err    error
	}{
		{
			name:   "libressl syscall",
			output: "fatal: unable to access 'https://github.com/github/awesome-copilot/': LibreSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443",
			err:    errors.New("exit status 128"),
		},
		{
			name:   "gnutls handshake",
			output: "fatal: unable to access 'https://github.com/example/repo/': GnuTLS recv error (-110): The TLS connection was non-properly terminated.",
			err:    errors.New("exit status 128"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !isTransientGitCloneError(tc.output, tc.err) {
				t.Fatalf("Git clone SSL 断线应判定为可重试: %s", tc.output)
			}
		})
	}
}

func TestRepairClaudePluginsTextFixesMojibake(t *testing.T) {
	chinese := repairClaudePluginsText("\u00e9\u009b\u0086\u00e6\u0088\u0090\u00e9\u00a3\u009e\u00e4\u00b9\u00a6/Feishu \u00e6\u009c\u008d\u00e5\u008a\u00a1")
	if chinese != "集成飞书/Feishu 服务" {
		t.Fatalf("中文乱码修复不正确: %q", chinese)
	}
	dash := repairClaudePluginsText("ready-marker contract \u00e2\u0080\u0094 designed for AI agents")
	if dash != "ready-marker contract — designed for AI agents" {
		t.Fatalf("符号乱码修复不正确: %q", dash)
	}
	normal := repairClaudePluginsText("Lark/Feishu API integration")
	if normal != "Lark/Feishu API integration" {
		t.Fatalf("正常描述不应被改写: %q", normal)
	}
}

func TestExtractPreviewMarkdownChoosesReadableHTMLFragment(t *testing.T) {
	body := `<html><script>{"dangerouslySetInnerHTML":{"__html":"{\"@context\":\"https://schema.org\"}"}}</script>` +
		`<script>{"dangerouslySetInnerHTML":{"__html":"\u003ch1\u003ePDF Skill\u003c/h1\u003e\u003cp\u003eRead PDFs.\u003c/p\u003e"}}</script></html>`

	markdown := extractPreviewMarkdown(body)
	if !strings.Contains(markdown, "# PDF Skill") || !strings.Contains(markdown, "Read PDFs.") || strings.Contains(markdown, "@context") {
		t.Fatalf("预览内容提取不正确: %q", markdown)
	}
}

func TestGetExternalSkillPreviewSkipsSkillsShBodyFetch(t *testing.T) {
	service := NewService(newSkillsTestConfig(t), nil, nil)

	preview, err := service.GetExternalSkillPreview(context.Background(), "https://skills.sh/zc277584121/marketing-skills/md-to-feishu")
	if err != nil {
		t.Fatalf("skills.sh 预览跳过失败: %v", err)
	}
	if preview.DetailURL != "https://www.skills.sh/zc277584121/marketing-skills/md-to-feishu" || preview.ReadmeMarkdown != "" {
		t.Fatalf("skills.sh 预览不应拉取正文: %+v", preview)
	}
}

func TestValidateExternalURLCanonicalizesSkillsShDetailHost(t *testing.T) {
	service := NewService(newSkillsTestConfig(t), nil, nil)

	targetURL, err := service.validateExternalURL(context.Background(), "https://skills.sh/zc277584121/marketing-skills/md-to-feishu")
	if err != nil {
		t.Fatalf("校验 skills.sh 详情链接失败: %v", err)
	}
	if targetURL != "https://www.skills.sh/zc277584121/marketing-skills/md-to-feishu" {
		t.Fatalf("skills.sh 详情链接未规范化: %s", targetURL)
	}
}

func TestImportLocalPathPersistsPrivateSourceMetadata(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)
	ctx := context.Background()

	sourceRoot := filepath.Join(t.TempDir(), "private-skill")
	writeTestSkillDir(t, sourceRoot, "private-skill", "Private Skill", false)
	detail, err := service.ImportLocalPath(ctx, sourceRoot)
	if err != nil {
		t.Fatalf("导入本地路径 skill 失败: %v", err)
	}
	if detail.SourceKind != externalSourceKindLocalPath || detail.SourceName != "本地路径" || detail.SourceTrust != externalSourceTrustPrivate {
		t.Fatalf("本地导入来源元数据不正确: %+v", detail.Info)
	}
	record, err := service.skillStore.GetImportedSkill(ctx, authctx.OwnerUserID(ctx), "private-skill")
	if err != nil {
		t.Fatalf("读取导入 skill 记录失败: %v", err)
	}
	if record == nil || record.ImportMode != externalSourceKindLocalPath || record.SourceName != "本地路径" || record.SourceTrust != externalSourceTrustPrivate {
		t.Fatalf("导入 skill DB 元数据不正确: %+v", record)
	}
}

func TestGitImportAndUpdateImportedSkillsUseStoredMetadata(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	service := NewServiceWithDB(cfg, db, nil, nil)
	ctx := context.Background()

	repoV1 := filepath.Join(t.TempDir(), "repo-v1")
	repoV2 := filepath.Join(t.TempDir(), "repo-v2")
	writeTestSkillDir(t, filepath.Join(repoV1, "skills", "git-skill"), "git-skill", "Git Skill v1", false)
	writeTestSkillDir(t, filepath.Join(repoV2, "skills", "git-skill"), "git-skill", "Git Skill v2", false)
	activeRepo := repoV1
	activeCommit := "commit-v1"
	service.commandRunner = func(_ context.Context, workDir string, _ []string, command ...string) (string, error) {
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "clone") {
			return "", copyDirectory(activeRepo, command[len(command)-1])
		}
		if len(command) >= 3 && command[0] == "git" && command[1] == "rev-parse" && workDir != "" {
			return activeCommit, nil
		}
		return "", errors.New("unexpected command")
	}

	detail, err := service.ImportGitPath(ctx, "https://example.com/skills.git", "main", "skills/git-skill")
	if err != nil {
		t.Fatalf("Git 导入失败: %v", err)
	}
	if detail.SourceKind != externalSourceKindGit || detail.ImportMode != externalSourceKindGit || detail.Version != "commit-v1" {
		t.Fatalf("Git 导入元数据不正确: %+v", detail.Info)
	}
	record, err := service.skillStore.GetImportedSkill(ctx, authctx.OwnerUserID(ctx), "git-skill")
	if err != nil {
		t.Fatalf("读取 Git 导入记录失败: %v", err)
	}
	if record == nil || record.GitURL != "https://example.com/skills.git" || record.GitBranch != "main" || record.GitPath != "skills/git-skill" {
		t.Fatalf("Git 导入 DB 记录不正确: %+v", record)
	}

	localRoot := filepath.Join(t.TempDir(), "local-skill")
	writeTestSkillDir(t, localRoot, "local-skill", "Local Skill", false)
	if _, err = service.ImportLocalPath(ctx, localRoot); err != nil {
		t.Fatalf("导入本地 skill 失败: %v", err)
	}

	activeRepo = repoV2
	activeCommit = "commit-v2"
	updateResult, err := service.UpdateImportedSkills(ctx)
	if err != nil {
		t.Fatalf("更新技能库失败: %v", err)
	}
	if !stringSliceContains(updateResult.UpdatedSkills, "git-skill") {
		t.Fatalf("Git skill 未被更新: %+v", updateResult)
	}
	if !stringSliceContains(updateResult.SkippedSkills, "local-skill") {
		t.Fatalf("本地导入 skill 应被跳过: %+v", updateResult)
	}
	updated, err := service.GetSkillDetail(ctx, "git-skill", "")
	if err != nil {
		t.Fatalf("读取更新后 Git skill 失败: %v", err)
	}
	if updated.Title != "Git Skill v2" || updated.Version != "commit-v2" {
		t.Fatalf("Git 更新后详情不正确: %+v", updated.Info)
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

func stringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func stringSliceContainsPrefix(items []string, prefix string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func buildTestSkillZip(t *testing.T, name string, title string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("skills/" + name + "/SKILL.md")
	if err != nil {
		t.Fatalf("创建测试 zip 条目失败: %v", err)
	}
	content := `---
name: ` + name + `
title: ` + title + `
description: Zip skill demo
---

# ` + title + `
`
	if _, err = file.Write([]byte(content)); err != nil {
		t.Fatalf("写入测试 zip 条目失败: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("关闭测试 zip 失败: %v", err)
	}
	return buffer.Bytes()
}
