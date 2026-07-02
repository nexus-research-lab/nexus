package skills

import (
	"context"
	"database/sql"
	"path/filepath"
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
		OwnerUserID: "owner-a",
		SourceID:    "skill_src_test",
		Name:        "Test Hub",
		Kind:        "well_known",
		URL:         "https://example.com/agentskills.json",
		Trust:       "community",
		Enabled:     true,
		SortOrder:   10,
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
	if storedSource == nil || storedSource.LastCheckedAt == nil || storedSource.LastError != "boom" {
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
	if len(items) != 1 || items[0].Version != "v2" || items[0].ContentHash != "hash-b" {
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
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("创建测试表失败: %v", err)
		}
	}
}
