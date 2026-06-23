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
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

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

func TestRunGitCloneAttemptDoesNotFallbackToMaster(t *testing.T) {
	service := &Service{}
	cloneCommands := [][]string{}
	service.commandRunner = func(_ context.Context, _ string, _ []string, command ...string) (string, error) {
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "ls-remote") {
			return "", errors.New("remote head unavailable")
		}
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "clone") {
			cloneCommands = append(cloneCommands, append([]string(nil), command...))
			return "fatal: repository not found", errors.New("exit status 128")
		}
		return "", errors.New("unexpected command")
	}

	output, err := service.runGitCloneAttempt(
		context.Background(),
		"https://example.com/skills.git",
		filepath.Join(t.TempDir(), "repo"),
		gitCloneOptions{},
	)
	if err == nil || !strings.Contains(output, "repository not found") {
		t.Fatalf("clone 失败应返回原始错误: output=%q err=%v", output, err)
	}
	if len(cloneCommands) != 1 {
		t.Fatalf("clone 不应额外 fallback 到 master: %+v", cloneCommands)
	}
	if stringSliceContains(cloneCommands[0], "--branch") || stringSliceContains(cloneCommands[0], "master") {
		t.Fatalf("remote HEAD 缺失时应让 git 使用默认 HEAD，不应指定 master: %+v", cloneCommands[0])
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

func stringSliceContains(items []string, target string) bool {
	return slices.Contains(items, target)
}

func stringSliceContainsPrefix(items []string, prefix string) bool {
	return slices.ContainsFunc(items, func(item string) bool {
		return strings.HasPrefix(item, prefix)
	})
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
