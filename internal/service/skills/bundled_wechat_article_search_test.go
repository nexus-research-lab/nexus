// 微信公众号文章搜索 Skill 的平台目录元数据回归测试。
package skills

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledWechatArticleSearchCatalogMetadata(t *testing.T) {
	const skillName = "wechat-article-search"

	service := &Service{}
	curatedEntries, err := service.loadCuratedEntries()
	if err != nil {
		t.Fatalf("读取精选 Skill 目录失败: %v", err)
	}
	record, err := service.buildBuiltinRecord(
		filepath.Join(projectRoot(), "skills", skillName),
		curatedEntries[skillName],
		sourceKindNexusPlatform,
	)
	if err != nil {
		t.Fatalf("读取微信文章搜索 Skill 失败: %v", err)
	}

	detail := record.Detail
	if detail.Name != skillName ||
		detail.Title != "微信公众号文章搜索" ||
		detail.Version != "1.0.0" {
		t.Fatalf("微信文章搜索 Skill 基础元数据不正确: %+v", detail.Info)
	}
	if detail.SourceType != sourceTypeBuiltin ||
		detail.SourceKind != sourceKindNexusPlatform ||
		detail.CategoryKey != "research-analysis" ||
		detail.CategoryName != "研究与分析" {
		t.Fatalf("微信文章搜索 Skill 目录分类不正确: %+v", detail.Info)
	}
	if !strings.Contains(detail.ReadmeMarkdown, "${CLAUDE_SKILL_DIR}/scripts/search.py") {
		t.Fatal("微信文章搜索 Skill 缺少可移植的脚本目录引用")
	}
}
