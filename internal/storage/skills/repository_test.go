package skills

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "modernc.org/sqlite"
)

func TestRepositoryStoresSourcesAndImportedSkills(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "skills.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	createSkillRepositoryTestSchema(t, db)

	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	ctx := context.Background()
	source := SourceEntity{
		OwnerUserID:          "owner-a",
		SourceID:             "skill_src_test",
		Name:                 "Test Hub",
		Kind:                 "well_known",
		URL:                  "https://example.com/agentskills.json",
		Trust:                "community",
		ManagedBy:            "user",
		AuthType:             "bearer",
		CredentialsEncrypted: "v1:test",
		Enabled:              true,
		SortOrder:            10,
	}
	if err = repository.EnsureSource(ctx, source); err != nil {
		t.Fatalf("写入来源失败: %v", err)
	}
	source.Name = "Ignored"
	source.Enabled = false
	if err = repository.EnsureSource(ctx, source); err != nil {
		t.Fatalf("重复 ensure 来源失败: %v", err)
	}
	sources, err := repository.ListEnabledSources(ctx, "owner-a")
	if err != nil {
		t.Fatalf("读取来源失败: %v", err)
	}
	if len(sources) != 1 || sources[0].Name != "Test Hub" {
		t.Fatalf("ensure 不应覆盖已有来源: %+v", sources)
	}
	checkedAt := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	if err = repository.RecordSourceCheck(ctx, "owner-a", source.SourceID, checkedAt, "boom"); err != nil {
		t.Fatalf("记录来源检查状态失败: %v", err)
	}
	storedSource, err := repository.GetSource(ctx, "owner-a", source.SourceID)
	if err != nil {
		t.Fatalf("读取来源详情失败: %v", err)
	}
	if storedSource == nil || storedSource.LastCheckedAt == nil || storedSource.LastError != "boom" ||
		storedSource.ManagedBy != "user" || storedSource.AuthType != "bearer" || storedSource.CredentialsEncrypted != "v1:test" {
		t.Fatalf("来源检查状态未写入: %+v", storedSource)
	}

	imported := ImportedSkillEntity{
		OwnerUserID:    "owner-a",
		SkillName:      "demo-skill",
		Title:          "Demo Skill",
		Scope:          "any",
		TagsJSON:       `["demo"]`,
		CategoryKey:    "custom-imports",
		CategoryName:   "自定义导入",
		Recommendation: "demo",
		Version:        "v1",
		SourceID:       source.SourceID,
		SourceKind:     source.Kind,
		SourceRef:      source.URL,
		SourceName:     "Test Hub",
		SourceTrust:    "community",
		SourceSkillID:  "demo-id",
		ArtifactSHA256: "artifact-a",
		ImportMode:     "url",
		RawURL:         "https://example.com/SKILL.md",
		ContentHash:    "hash-a",
	}
	if err = repository.UpsertImportedSkill(ctx, imported); err != nil {
		t.Fatalf("写入导入 skill 失败: %v", err)
	}
	imported.Version = "v2"
	imported.ContentHash = "hash-b"
	if err = repository.UpsertImportedSkill(ctx, imported); err != nil {
		t.Fatalf("更新导入 skill 失败: %v", err)
	}
	items, err := repository.ListImportedSkills(ctx, "owner-a")
	if err != nil {
		t.Fatalf("读取导入 skill 失败: %v", err)
	}
	if len(items) != 1 || items[0].Version != "v2" || items[0].ContentHash != "hash-b" ||
		items[0].SourceSkillID != "demo-id" || items[0].ArtifactSHA256 != "artifact-a" {
		t.Fatalf("导入 skill upsert 不正确: %+v", items)
	}
	if err = repository.RecordImportedSkillCheck(ctx, "owner-a", "demo-skill", true, checkedAt, ""); err != nil {
		t.Fatalf("记录导入 skill 检查状态失败: %v", err)
	}
	checkedSkill, err := repository.GetImportedSkill(ctx, "owner-a", "demo-skill")
	if err != nil {
		t.Fatalf("读取检查后的导入 skill 失败: %v", err)
	}
	if checkedSkill == nil || !checkedSkill.UpdateAvailable || checkedSkill.LastCheckedAt == nil {
		t.Fatalf("导入 skill 检查状态未写入: %+v", checkedSkill)
	}
}

func TestCatalogMutationVersionCASAndRollback(t *testing.T) {
	db, err := sql.Open(
		"sqlite",
		filepath.Join(t.TempDir(), "skills-version.db")+"?_pragma=busy_timeout(5000)",
	)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	createSkillRepositoryTestSchema(t, db)
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	ctx := context.Background()

	version, err := repository.CatalogVersion(ctx, "owner-a")
	if err != nil || version != 1 {
		t.Fatalf("初始 catalog version = %d, err=%v, want 1", version, err)
	}
	expected := int64(1)
	mutation, err := repository.BeginCatalogMutation(ctx, "owner-a", &expected, true)
	if err != nil {
		t.Fatalf("开始 catalog mutation 失败: %v", err)
	}
	if mutation.Version() != 2 {
		t.Fatalf("transaction version = %d, want 2", mutation.Version())
	}
	if err = mutation.UpsertSource(ctx, SourceEntity{
		OwnerUserID: "owner-a",
		SourceID:    "source-a",
		Name:        "Source A",
		Kind:        "url",
		URL:         "https://example.com/a",
		Trust:       "community",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("mutation 写来源失败: %v", err)
	}
	if err = mutation.Commit(); err != nil {
		t.Fatalf("提交 catalog mutation 失败: %v", err)
	}
	version, err = repository.CatalogVersion(ctx, "owner-a")
	if err != nil || version != 2 {
		t.Fatalf("提交后 catalog version = %d, err=%v, want 2", version, err)
	}
	if _, err = repository.BeginCatalogMutation(ctx, "owner-a", &expected, true); !errors.Is(err, ErrCatalogVersionConflict) {
		t.Fatalf("过期 expected version error = %v, want conflict", err)
	}

	expected = 2
	mutation, err = repository.BeginCatalogMutation(ctx, "owner-a", &expected, true)
	if err != nil {
		t.Fatalf("开始待回滚 mutation 失败: %v", err)
	}
	source, err := mutation.GetSource(ctx, "source-a")
	if err != nil || source == nil {
		t.Fatalf("transaction 读取来源失败: source=%+v err=%v", source, err)
	}
	source.Enabled = false
	if err = mutation.UpsertSource(ctx, *source); err != nil {
		t.Fatalf("transaction 更新来源失败: %v", err)
	}
	if err = mutation.Rollback(); err != nil {
		t.Fatalf("回滚 catalog mutation 失败: %v", err)
	}
	version, err = repository.CatalogVersion(ctx, "owner-a")
	if err != nil || version != 2 {
		t.Fatalf("回滚后 catalog version = %d, err=%v, want 2", version, err)
	}
	stored, err := repository.GetSource(ctx, "owner-a", "source-a")
	if err != nil || stored == nil || !stored.Enabled {
		t.Fatalf("回滚后来源不正确: source=%+v err=%v", stored, err)
	}
}

func TestCatalogMutationConcurrentCASHasSingleWinner(t *testing.T) {
	db, err := sql.Open(
		"sqlite",
		filepath.Join(t.TempDir(), "skills-race.db")+"?_pragma=busy_timeout(5000)",
	)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	createSkillRepositoryTestSchema(t, db)
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	ctx := context.Background()
	if _, err = repository.CatalogVersion(ctx, "owner-a"); err != nil {
		t.Fatalf("初始化 catalog version 失败: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			expected := int64(1)
			mutation, beginErr := repository.BeginCatalogMutation(
				ctx,
				"owner-a",
				&expected,
				true,
			)
			if beginErr != nil {
				results <- beginErr
				return
			}
			results <- mutation.Commit()
		}()
	}
	close(start)
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, ErrCatalogVersionConflict):
			conflicts++
		default:
			t.Fatalf("并发 CAS 返回未知错误: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("并发 CAS successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
	version, err := repository.CatalogVersion(ctx, "owner-a")
	if err != nil || version != 2 {
		t.Fatalf("并发 CAS 后 version=%d err=%v, want 2", version, err)
	}
}

func createSkillRepositoryTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE skill_sources (
			owner_user_id VARCHAR(64) NOT NULL,
			source_id VARCHAR(64) NOT NULL,
			name VARCHAR(255) NOT NULL,
			kind VARCHAR(32) NOT NULL,
			url TEXT NOT NULL,
			trust VARCHAR(32) NOT NULL DEFAULT 'community',
			managed_by VARCHAR(32) NOT NULL DEFAULT 'system',
			auth_type VARCHAR(32) NOT NULL DEFAULT 'none',
			credentials_encrypted TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT 1,
			sort_order INTEGER NOT NULL DEFAULT 100,
			last_checked_at DATETIME,
			last_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
			PRIMARY KEY (owner_user_id, source_id),
			UNIQUE (owner_user_id, kind, url)
		)`,
		`CREATE TABLE imported_skills (
			owner_user_id VARCHAR(64) NOT NULL,
			skill_name VARCHAR(255) NOT NULL,
			title VARCHAR(255) NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			scope VARCHAR(32) NOT NULL DEFAULT 'any',
			tags TEXT NOT NULL DEFAULT '[]',
			category_key VARCHAR(128) NOT NULL DEFAULT 'custom-imports',
			category_name VARCHAR(128) NOT NULL DEFAULT '自定义导入',
			recommendation TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			source_id VARCHAR(64) NOT NULL DEFAULT '',
			source_kind VARCHAR(32) NOT NULL DEFAULT '',
			source_ref TEXT NOT NULL DEFAULT '',
			source_name VARCHAR(255) NOT NULL DEFAULT '',
			source_trust VARCHAR(32) NOT NULL DEFAULT 'community',
			source_skill_id VARCHAR(255) NOT NULL DEFAULT '',
			artifact_sha256 VARCHAR(64) NOT NULL DEFAULT '',
			import_mode VARCHAR(32) NOT NULL DEFAULT '',
			git_url TEXT NOT NULL DEFAULT '',
			git_branch VARCHAR(255) NOT NULL DEFAULT '',
			git_path TEXT NOT NULL DEFAULT '',
			git_commit VARCHAR(128) NOT NULL DEFAULT '',
			raw_url TEXT NOT NULL DEFAULT '',
			detail_url TEXT NOT NULL DEFAULT '',
			content_hash VARCHAR(128) NOT NULL DEFAULT '',
			update_available BOOLEAN NOT NULL DEFAULT 0,
			last_imported_at DATETIME,
			last_checked_at DATETIME,
			last_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
			PRIMARY KEY (owner_user_id, skill_name)
		)`,
		`CREATE TABLE skill_catalog_versions (
			owner_user_id TEXT PRIMARY KEY,
			version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("创建测试表失败: %v", err)
		}
	}
}
