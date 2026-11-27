// marketplace_test.go 覆盖外部与私有 Skill 市场同步测试。
package skills

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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

func TestPrivateRegistrySourceSearchImportAndUpdate(t *testing.T) {
	const token = "private-token"
	const secondOwnerToken = "second-owner-token"
	var stateMutex sync.RWMutex
	archive := buildTestSkillZipEntry(
		t,
		"internal-knowledge/SKILL.md",
		"internal-knowledge",
		"Internal Knowledge v1",
	)
	skillName := "internal-knowledge"
	version := "1.0.0"
	var publicSearchCalls atomic.Int32

	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/registry/api/skills", func(writer http.ResponseWriter, request *http.Request) {
		authorization := request.Header.Get("Authorization")
		if authorization != "Bearer "+token && authorization != "Bearer "+secondOwnerToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		stateMutex.RLock()
		currentArchive := append([]byte(nil), archive...)
		currentSkillName := skillName
		currentVersion := version
		stateMutex.RUnlock()
		sum := sha256.Sum256(currentArchive)
		payload := privateRegistryResponse{
			Skills: []privateRegistrySkill{{
				ID:             "internal-knowledge",
				Name:           currentSkillName,
				Title:          "Internal Knowledge",
				Description:    "Private knowledge guidance",
				Version:        currentVersion,
				Tags:           []string{"knowledge", "private"},
				DownloadURL:    server.URL + "/registry/download/internal-knowledge.zip",
				SHA256:         hex.EncodeToString(sum[:]),
				Size:           int64(len(currentArchive)),
				ReadmeMarkdown: "# Internal Knowledge",
			}},
			Total: 1,
		}
		if requestedID := request.URL.Query().Get("id"); requestedID != "" && requestedID != "internal-knowledge" {
			payload.Skills = []privateRegistrySkill{}
			payload.Total = 0
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(payload); err != nil {
			t.Errorf("写入私有来源响应失败: %v", err)
		}
	})
	mux.HandleFunc("/registry/download/internal-knowledge.zip", func(writer http.ResponseWriter, request *http.Request) {
		authorization := request.Header.Get("Authorization")
		if authorization != "Bearer "+token && authorization != "Bearer "+secondOwnerToken {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		stateMutex.RLock()
		currentArchive := append([]byte(nil), archive...)
		stateMutex.RUnlock()
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = writer.Write(currentArchive)
	})
	mux.HandleFunc("/public-index.json", func(writer http.ResponseWriter, _ *http.Request) {
		publicSearchCalls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"skills":[]}`))
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := newSkillsTestConfig(t)
	cfg.SkillsDefaultSourcesEnabled = false
	cfg.SkillsAPIURL = ""
	cfg.SkillsSourceURLs = "Public Test|" + server.URL + "/public-index.json"
	migrateSkillsSQLite(t, cfg.DatabaseURL)
	db := openSkillsTestDB(t, cfg)
	service := NewServiceWithDB(cfg, db, nil, nil)
	ctx := context.Background()
	beforeCreate, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取创建前 catalog version 失败: %v", err)
	}

	source, err := service.CreateExternalSkillSource(ctx, CreateExternalSkillSourceRequest{
		Name:     "Internal Registry",
		URL:      server.URL + "/registry",
		AuthType: externalSourceAuthBearer,
		Token:    token,
	})
	if err != nil {
		t.Fatalf("创建私有来源失败: %v", err)
	}
	if source.ManagedBy != externalSourceManagedByUser || !source.CredentialConfigured || !source.Deletable {
		t.Fatalf("私有来源公开状态不正确: %+v", source)
	}
	afterCreate, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取创建后 catalog version 失败: %v", err)
	}
	if afterCreate.Version != beforeCreate.Version+1 {
		t.Fatalf("创建私有来源 catalog version = %d, want %d", afterCreate.Version, beforeCreate.Version+1)
	}
	storedSource, err := service.skillStore.GetSource(ctx, authctx.OwnerUserID(ctx), source.SourceID)
	if err != nil {
		t.Fatalf("读取私有来源记录失败: %v", err)
	}
	if storedSource == nil || storedSource.CredentialsEncrypted != token {
		t.Fatalf("私有来源 Token 未按明文持久化: %+v", storedSource)
	}
	secondOwnerContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "owner-b"})
	secondOwnerSource, err := service.CreateExternalSkillSource(secondOwnerContext, CreateExternalSkillSourceRequest{
		Name:     "Internal Registry B",
		URL:      server.URL + "/registry",
		AuthType: externalSourceAuthBearer,
		Token:    secondOwnerToken,
	})
	if err != nil {
		t.Fatalf("第二个 owner 创建同源私有来源失败: %v", err)
	}
	if secondOwnerSource.SourceID != source.SourceID {
		t.Fatalf("同一 URL 应生成稳定 source_id: %s != %s", secondOwnerSource.SourceID, source.SourceID)
	}
	secondOwnerStoredSource, err := service.skillStore.GetSource(secondOwnerContext, "owner-b", source.SourceID)
	if err != nil {
		t.Fatalf("读取第二个 owner 来源失败: %v", err)
	}
	if secondOwnerStoredSource == nil || secondOwnerStoredSource.CredentialsEncrypted != secondOwnerToken {
		t.Fatalf("不同 owner 的来源凭据未隔离: %+v", secondOwnerStoredSource)
	}

	result, err := service.SearchExternalSkillsFromSource(ctx, "knowledge", false, source.SourceID)
	if err != nil {
		t.Fatalf("搜索私有来源失败: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].PackageSpec != "internal-knowledge" || result.Results[0].SourceKind != externalSourceKindPrivateRegistry {
		t.Fatalf("私有来源搜索结果不正确: %+v", result.Results)
	}
	if publicSearchCalls.Load() != 0 {
		t.Fatalf("定向私有搜索不应请求公共来源，实际请求 %d 次", publicSearchCalls.Load())
	}

	beforeImport, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取导入前 catalog version 失败: %v", err)
	}
	detail, err := service.ImportPrivateSkillFromSource(ctx, ImportPrivateSkillRequest{
		SourceID: source.SourceID,
		SkillID:  "internal-knowledge",
	})
	if err != nil {
		t.Fatalf("导入私有 skill 失败: %v", err)
	}
	if detail.Name != "internal-knowledge" || detail.Title != "Internal Knowledge v1" {
		t.Fatalf("私有 skill 详情不正确: %+v", detail.Info)
	}
	afterImport, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取导入后 catalog version 失败: %v", err)
	}
	if afterImport.Version != beforeImport.Version+1 {
		t.Fatalf("导入私有 Skill catalog version = %d, want %d", afterImport.Version, beforeImport.Version+1)
	}
	record, err := service.skillStore.GetImportedSkill(ctx, authctx.OwnerUserID(ctx), detail.Name)
	if err != nil {
		t.Fatalf("读取私有 skill 记录失败: %v", err)
	}
	if record == nil || record.SourceSkillID != "internal-knowledge" || record.ArtifactSHA256 == "" {
		t.Fatalf("私有 skill 更新元数据未持久化: %+v", record)
	}

	stateMutex.Lock()
	archive = buildTestSkillZipEntry(
		t,
		"internal-knowledge/SKILL.md",
		"internal-knowledge",
		"Internal Knowledge v2",
	)
	version = "2.0.0"
	stateMutex.Unlock()
	beforeCheck, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取健康检查前 catalog version 失败: %v", err)
	}
	updates, err := service.CheckImportedSkillUpdates(ctx)
	if err != nil {
		t.Fatalf("检查私有 skill 更新失败: %v", err)
	}
	if len(updates.AvailableSkills) != 1 || updates.AvailableSkills[0] != "internal-knowledge" {
		t.Fatalf("未发现私有 skill 更新: %+v", updates)
	}
	afterCheck, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取健康检查后 catalog version 失败: %v", err)
	}
	if afterCheck.Version != beforeCheck.Version {
		t.Fatalf("非功能健康检查不应推进 catalog version: %d != %d", afterCheck.Version, beforeCheck.Version)
	}
	updated, err := service.UpdateSingleSkill(ctx, "internal-knowledge")
	if err != nil {
		t.Fatalf("更新私有 skill 失败: %v", err)
	}
	if updated.Title != "Internal Knowledge v2" {
		t.Fatalf("私有 skill 未更新到最新内容: %+v", updated.Info)
	}
	stateMutex.Lock()
	skillName = "renamed-internal-knowledge"
	stateMutex.Unlock()
	invalidUpdate, err := service.CheckImportedSkillUpdates(ctx)
	if err != nil {
		t.Fatalf("检查变更名称的私有 skill 更新失败: %v", err)
	}
	if len(invalidUpdate.Failures) != 1 || invalidUpdate.Failures[0].SkillName != "internal-knowledge" {
		t.Fatalf("私有来源不得通过原 id 变更 skill name: %+v", invalidUpdate)
	}
	stateMutex.Lock()
	skillName = "internal-knowledge"
	stateMutex.Unlock()

	beforeDelete, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取删除前 catalog version 失败: %v", err)
	}
	if err = service.DeleteExternalSkillSource(ctx, source.SourceID); err != nil {
		t.Fatalf("删除私有来源失败: %v", err)
	}
	afterDelete, err := service.GetCatalogState(ctx)
	if err != nil {
		t.Fatalf("读取删除后 catalog version 失败: %v", err)
	}
	if afterDelete.Version != beforeDelete.Version+1 {
		t.Fatalf("删除私有来源 catalog version = %d, want %d", afterDelete.Version, beforeDelete.Version+1)
	}
	sources, err := service.ListExternalSkillSources(ctx)
	if err != nil {
		t.Fatalf("读取删除后的来源失败: %v", err)
	}
	if len(sources) != 1 || sources[0].ManagedBy != externalSourceManagedBySystem {
		t.Fatalf("私有来源未删除: %+v", sources)
	}
	restoredSource, err := service.CreateExternalSkillSource(ctx, CreateExternalSkillSourceRequest{
		Name:     "Internal Registry Restored",
		URL:      server.URL + "/registry",
		AuthType: externalSourceAuthBearer,
		Token:    token,
	})
	if err != nil {
		t.Fatalf("重新添加私有来源失败: %v", err)
	}
	if restoredSource.SourceID != source.SourceID {
		t.Fatalf("重新添加同一 URL 应恢复原 source_id: %s != %s", restoredSource.SourceID, source.SourceID)
	}
	restoredUpdates, err := service.CheckImportedSkillUpdates(ctx)
	if err != nil {
		t.Fatalf("恢复来源后检查更新失败: %v", err)
	}
	if len(restoredUpdates.Failures) != 0 || len(restoredUpdates.AvailableSkills) != 0 {
		t.Fatalf("恢复来源后应继续识别当前版本: %+v", restoredUpdates)
	}
	secondOwnerSources, err := service.ListExternalSkillSources(secondOwnerContext)
	if err != nil {
		t.Fatalf("读取第二个 owner 来源失败: %v", err)
	}
	if len(secondOwnerSources) != 2 {
		t.Fatalf("删除第一个 owner 来源不应影响第二个 owner: %+v", secondOwnerSources)
	}
}

func TestValidatePrivateRegistrySkillSourceRequiresExplicitTopLevelName(t *testing.T) {
	testCases := []struct {
		name        string
		relativeDir string
		content     string
		wantError   bool
	}{
		{
			name:        "valid",
			relativeDir: "internal-knowledge",
			content:     "---\nname: internal-knowledge\n---\n# Internal Knowledge\n",
		},
		{
			name:        "missing frontmatter name",
			relativeDir: "internal-knowledge",
			content:     "---\ndescription: private\n---\n# Internal Knowledge\n",
			wantError:   true,
		},
		{
			name:        "nested skill directory",
			relativeDir: filepath.Join("wrapper", "internal-knowledge"),
			content:     "---\nname: internal-knowledge\n---\n# Internal Knowledge\n",
			wantError:   true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			sourceDir := filepath.Join(root, testCase.relativeDir)
			if err := os.MkdirAll(sourceDir, 0o755); err != nil {
				t.Fatalf("创建测试目录失败: %v", err)
			}
			if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte(testCase.content), 0o644); err != nil {
				t.Fatalf("写入测试 Skill 失败: %v", err)
			}
			err := validatePrivateRegistrySkillSource(root, sourceDir, "internal-knowledge")
			if (err != nil) != testCase.wantError {
				t.Fatalf("校验结果不符合预期: err=%v", err)
			}
		})
	}
}

func TestPrivateRegistrySourceRequiresAllowedHostInAuthenticatedDeployment(t *testing.T) {
	service := NewService(newSkillsTestConfig(t), nil, nil)
	ctx := authctx.WithState(context.Background(), authctx.State{AuthRequired: true})

	if _, err := service.validatePrivateRegistryBaseURL(ctx, "https://skills.example.com"); err == nil {
		t.Fatal("认证部署不应接受未进入白名单的私有来源")
	}
	service.config.SkillsPrivateSourceAllowedHosts = []string{"skills.example.com"}
	if _, err := service.validatePrivateRegistryBaseURL(ctx, "https://skills.example.com/registry"); err != nil {
		t.Fatalf("认证部署应接受白名单中的私有来源: %v", err)
	}
}
