package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

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
