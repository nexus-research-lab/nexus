// 专业图解设计 Skill 的来源、许可与平台目录元数据回归测试。
package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledDiagramDesignCatalogAndAttribution(t *testing.T) {
	const (
		skillName  = "diagram-design"
		upstream   = "https://github.com/cathrynlavery/diagram-design"
		upstreamID = "a157f7616473d966d6f433cf0b4d4f1880603504"
	)

	service := &Service{}
	curatedEntries, err := service.loadCuratedEntries()
	if err != nil {
		t.Fatalf("读取精选 Skill 目录失败: %v", err)
	}
	skillRoot := filepath.Join(projectRoot(), "skills", skillName)
	record, err := service.buildBuiltinRecord(
		skillRoot,
		curatedEntries[skillName],
		sourceKindNexusPlatform,
	)
	if err != nil {
		t.Fatalf("读取专业图解设计 Skill 失败: %v", err)
	}

	detail := record.Detail
	if detail.Name != skillName ||
		detail.Title != "专业图解设计" ||
		detail.Version != "2.0.0" {
		t.Fatalf("专业图解设计 Skill 基础元数据不正确: %+v", detail.Info)
	}
	if detail.SourceType != sourceTypeBuiltin ||
		detail.SourceKind != sourceKindNexusPlatform ||
		detail.CategoryKey != "design-frontend" ||
		detail.CategoryName != "设计与前端" {
		t.Fatalf("专业图解设计 Skill 目录分类不正确: %+v", detail.Info)
	}
	for _, marker := range []string{upstream, upstreamID, "MIT", "SOURCE.md", "LICENSE"} {
		if !strings.Contains(detail.ReadmeMarkdown, marker) {
			t.Fatalf("专业图解设计 Skill 缺少来源标记 %q", marker)
		}
	}

	licenseBody, err := os.ReadFile(filepath.Join(skillRoot, "LICENSE"))
	if err != nil {
		t.Fatalf("读取上游许可证失败: %v", err)
	}
	if !strings.Contains(string(licenseBody), "Copyright (c) 2025 Cathryn Lavery") ||
		!strings.Contains(string(licenseBody), "MIT License") {
		t.Fatal("专业图解设计 Skill 未保留完整上游 MIT 版权信息")
	}
	sourceBody, err := os.ReadFile(filepath.Join(skillRoot, "SOURCE.md"))
	if err != nil {
		t.Fatalf("读取专业图解设计 Skill 来源说明失败: %v", err)
	}
	for _, marker := range []string{upstream, upstreamID, "MIT", "Copyright (c) 2025 Cathryn Lavery"} {
		if !strings.Contains(string(sourceBody), marker) {
			t.Fatalf("专业图解设计 Skill 来源说明缺少 %q", marker)
		}
	}
	thirdPartyBody, err := os.ReadFile(filepath.Join(skillRoot, "THIRD_PARTY_LICENSES.md"))
	if err != nil {
		t.Fatalf("读取专业图解设计 Skill 第三方内容说明失败: %v", err)
	}
	for _, marker := range []string{"primitive-icons.md", "icons.html", "Google Fonts", "Playwright"} {
		if !strings.Contains(string(thirdPartyBody), marker) {
			t.Fatalf("专业图解设计 Skill 第三方内容说明缺少 %q", marker)
		}
	}
	for _, path := range []string{
		filepath.Join(skillRoot, "references", "type-architecture.md"),
		filepath.Join(skillRoot, "references", "primitive-icons.md"),
		filepath.Join(skillRoot, "assets", "template.html"),
		filepath.Join(skillRoot, "assets", "icons.html"),
		filepath.Join(skillRoot, "assets", "icons", "tabler", "database.svg"),
		filepath.Join(skillRoot, "assets", "icons", "simple", "kubernetes.svg"),
		filepath.Join(skillRoot, "assets", "icons", "url", "hop.svg"),
		filepath.Join(skillRoot, "assets", "example-sequence.html"),
		filepath.Join(skillRoot, "THIRD_PARTY_LICENSES.upstream.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("专业图解设计 Skill 缺少引用资源 %s: %v", path, err)
		}
	}
	rawIconCount := 0
	err = filepath.Walk(filepath.Join(skillRoot, "assets", "icons"), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".svg") {
			rawIconCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描专业图解设计原始 SVG 失败: %v", err)
	}
	if rawIconCount != 86 {
		t.Fatalf("专业图解设计原始 SVG 数量 = %d, want 86", rawIconCount)
	}
	for _, marker := range []string{"Apache Hop", "Pentaho", "Dagster", "Stata", "使用前核验许可证"} {
		if !strings.Contains(string(thirdPartyBody), marker) {
			t.Fatalf("专业图解设计 Skill 未保留图标授权警告 %q", marker)
		}
	}
}
