package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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
	db := openSkillsTestDB(t, cfg)
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
	if detail.Name != "url-demo" || detail.HasUpdate {
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

func TestPreviewAndImportSkillURLSupportBackslashZipEntries(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	archive := buildTestSkillZipEntry(t, "target-industry-customer-analysis\\SKILL.md", "target-industry-customer-analysis", "Target Industry Customer Analysis", "target-industry-customer-analysis\\assets\\", "target-industry-customer-analysis\\assets\\templates\\research-notes.schema.json")
	mux.HandleFunc("/target-industry-customer-analysis.zip", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(archive)
	})

	cfg := newSkillsTestConfig(t)
	cfg.SkillsDefaultSourcesEnabled = false
	cfg.SkillsSourceURLs = "Local Zip|" + server.URL + "/target-industry-customer-analysis.zip"
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db := openSkillsTestDB(t, cfg)
	service := NewServiceWithDB(cfg, db, nil, nil)
	downloadURL := server.URL + "/target-industry-customer-analysis.zip"

	preview, err := service.GetExternalSkillPreview(context.Background(), downloadURL)
	if err != nil {
		t.Fatalf("反斜杠 zip 预览失败: %v", err)
	}
	if !strings.Contains(preview.ReadmeMarkdown, "# Target Industry Customer Analysis") {
		t.Fatalf("反斜杠 zip 预览内容不正确: %+v", preview)
	}

	detail, err := service.ImportSkillURL(context.Background(), downloadURL, externalManifest{})
	if err != nil {
		t.Fatalf("反斜杠 zip URL 导入失败: %v", err)
	}
	if detail.Name != "target-industry-customer-analysis" || detail.Title != "Target Industry Customer Analysis" {
		t.Fatalf("反斜杠 zip URL 导入详情不正确: %+v", detail.Info)
	}
}

func TestImportSkillsShClonesRepositoryAndSelectsRequestedSkill(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db := openSkillsTestDB(t, cfg)
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
	db := openSkillsTestDB(t, cfg)
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

	builtinCollisionRoot := filepath.Join(t.TempDir(), "builtin-collision")
	writeTestSkillDir(t, builtinCollisionRoot, "IMAGEGEN", "Builtin Collision", false)
	if _, err = service.ImportLocalPath(ctx, builtinCollisionRoot); err == nil {
		t.Fatal("外部 Skill 不应通过大小写变化覆盖系统 Skill")
	}
	caseCollisionRoot := filepath.Join(t.TempDir(), "case-collision")
	writeTestSkillDir(t, caseCollisionRoot, "PRIVATE-SKILL", "Case Collision", false)
	if _, err = service.ImportLocalPath(ctx, caseCollisionRoot); err == nil {
		t.Fatal("外部 Skill 不应创建仅大小写不同的重复源")
	}
}

func TestImportLocalPathRejectsAuthenticatedHostPath(t *testing.T) {
	service := NewService(newSkillsTestConfig(t), nil, nil)
	ctx := authctx.WithState(context.Background(), authctx.State{AuthRequired: true})
	sourceRoot := filepath.Join(t.TempDir(), "private-skill")
	writeTestSkillDir(t, sourceRoot, "private-skill", "Private Skill", false)

	if _, err := service.ImportLocalPath(ctx, sourceRoot); !errors.Is(err, ErrLocalPathImportUnavailable) {
		t.Fatalf("认证部署应拒绝宿主 local_path: %v", err)
	}
}

func TestGitImportAndUpdateImportedSkillsUseStoredMetadata(t *testing.T) {
	cfg := newSkillsTestConfig(t)
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db := openSkillsTestDB(t, cfg)
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
		if len(command) >= 2 && command[0] == "git" && stringSliceContains(command, "ls-remote") {
			return activeCommit + "\trefs/heads/main", nil
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
	if err = os.Remove(filepath.Join(service.registryRoot(ctx), "git-skill", ".nexus-skill.json")); err != nil {
		t.Fatalf("删除缓存 manifest 失败: %v", err)
	}
	initialSkills, err := service.ListSkills(ctx, Query{})
	if err != nil {
		t.Fatalf("读取初始 skill 列表失败: %v", err)
	}
	initialGitSkill, ok := findSkill(initialSkills, "git-skill")
	if !ok || initialGitSkill.HasUpdate {
		t.Fatalf("检查前不应显示有更新: %+v", initialGitSkill)
	}

	localRoot := filepath.Join(t.TempDir(), "local-skill")
	writeTestSkillDir(t, localRoot, "local-skill", "Local Skill", false)
	if _, err = service.ImportLocalPath(ctx, localRoot); err != nil {
		t.Fatalf("导入本地 skill 失败: %v", err)
	}

	activeRepo = repoV2
	activeCommit = "commit-v2"
	checkResult, err := service.CheckImportedSkillUpdates(ctx)
	if err != nil {
		t.Fatalf("检查技能更新失败: %v", err)
	}
	if !stringSliceContains(checkResult.AvailableSkills, "git-skill") {
		t.Fatalf("Git skill 应检查出可更新: %+v", checkResult)
	}
	if !stringSliceContains(checkResult.SkippedSkills, "local-skill") {
		t.Fatalf("本地导入 skill 应被检查跳过: %+v", checkResult)
	}
	checkedSkills, err := service.ListSkills(ctx, Query{})
	if err != nil {
		t.Fatalf("读取检查后 skill 列表失败: %v", err)
	}
	checkedGitSkill, ok := findSkill(checkedSkills, "git-skill")
	if !ok || !checkedGitSkill.HasUpdate {
		t.Fatalf("远端变化后应显示有更新: %+v", checkedGitSkill)
	}
	catalogBeforeUpdate, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取批量更新前 catalog version 失败: %v", err)
	}
	updateResult, err := service.UpdateImportedSkillsAtVersion(ctx, catalogBeforeUpdate.Version)
	if err != nil {
		t.Fatalf("更新技能库失败: %v", err)
	}
	if !stringSliceContains(updateResult.UpdatedSkills, "git-skill") {
		t.Fatalf("Git skill 未被更新: %+v", updateResult)
	}
	if !stringSliceContains(updateResult.SkippedSkills, "local-skill") {
		t.Fatalf("本地导入 skill 应被跳过: %+v", updateResult)
	}
	catalogAfterUpdate, err := service.GetCatalogState(ctx)
	if err != nil || catalogAfterUpdate.Version != catalogBeforeUpdate.Version+1 {
		t.Fatalf(
			"批量更新应只为成功发布的 Skill 推进版本: before=%+v after=%+v err=%v",
			catalogBeforeUpdate,
			catalogAfterUpdate,
			err,
		)
	}
	if _, err = service.UpdateImportedSkillsAtVersion(ctx, catalogBeforeUpdate.Version); !errors.Is(err, ErrCatalogVersionConflict) {
		t.Fatalf("批量更新必须拒绝过期 catalog version: %v", err)
	}
	updated, err := service.GetSkillDetail(ctx, "git-skill", "")
	if err != nil {
		t.Fatalf("读取更新后 Git skill 失败: %v", err)
	}
	if updated.Title != "Git Skill v2" || updated.Version != "commit-v2" {
		t.Fatalf("Git 更新后详情不正确: %+v", updated.Info)
	}
	refreshedSkills, err := service.ListSkills(ctx, Query{})
	if err != nil {
		t.Fatalf("读取更新后 skill 列表失败: %v", err)
	}
	refreshedGitSkill, ok := findSkill(refreshedSkills, "git-skill")
	if !ok || refreshedGitSkill.HasUpdate {
		t.Fatalf("更新后应清除有更新标记: %+v", refreshedGitSkill)
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

	return buildTestSkillZipEntry(t, "skills/"+name+"/SKILL.md", name, title)
}

func buildTestSkillZipEntry(t *testing.T, entryName string, name string, title string, extraEntries ...string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, extraEntry := range extraEntries {
		if _, err := writer.Create(extraEntry); err != nil {
			t.Fatalf("创建测试 zip 附加条目失败: %v", err)
		}
	}
	file, err := writer.Create(entryName)
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
