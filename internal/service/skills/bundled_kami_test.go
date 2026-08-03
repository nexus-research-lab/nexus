// Kami Skill 的来源、许可、依赖与平台目录元数据回归测试。
package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledKamiCatalogAndAttribution(t *testing.T) {
	const (
		skillName  = "kami"
		upstream   = "https://github.com/tw93/Kami"
		upstreamID = "a211e7bbf9470493debfbbf0fe4645c05eaa546f"
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
		t.Fatalf("读取 Kami Skill 失败: %v", err)
	}

	detail := record.Detail
	if detail.Name != skillName || detail.Title != "Kami 文档设计" || detail.Version != "1.11.0" {
		t.Fatalf("Kami Skill 基础元数据不正确: %+v", detail.Info)
	}
	if detail.SourceType != sourceTypeBuiltin ||
		detail.SourceKind != sourceKindNexusPlatform ||
		detail.CategoryKey != "content-docs" ||
		detail.CategoryName != "内容与文档" {
		t.Fatalf("Kami Skill 目录分类不正确: %+v", detail.Info)
	}
	for _, marker := range []string{upstream, upstreamID, "MIT", "SOURCE.md", "THIRD_PARTY_LICENSES.md"} {
		if !strings.Contains(detail.ReadmeMarkdown, marker) {
			t.Fatalf("Kami Skill 缺少来源标记 %q", marker)
		}
	}

	licenseBody, err := os.ReadFile(filepath.Join(skillRoot, "LICENSE"))
	if err != nil {
		t.Fatalf("读取 Kami 上游许可证失败: %v", err)
	}
	if !strings.Contains(string(licenseBody), "Copyright (c) 2026 Tw93") ||
		!strings.Contains(string(licenseBody), "MIT License") {
		t.Fatal("Kami Skill 未保留完整上游 MIT 版权信息")
	}

	sourceBody, err := os.ReadFile(filepath.Join(skillRoot, "SOURCE.md"))
	if err != nil {
		t.Fatalf("读取 Kami 来源说明失败: %v", err)
	}
	for _, marker := range []string{upstream, upstreamID, "1.11.0", "Copyright (c) 2026 Tw93"} {
		if !strings.Contains(string(sourceBody), marker) {
			t.Fatalf("Kami 来源说明缺少 %q", marker)
		}
	}

	for _, path := range []string{
		filepath.Join(skillRoot, "assets", "templates", "one-pager.html"),
		filepath.Join(skillRoot, "assets", "templates", "landing-page.html"),
		filepath.Join(skillRoot, "assets", "diagrams", "architecture-board.html"),
		filepath.Join(skillRoot, "references", "schemas", "resume.json"),
		filepath.Join(skillRoot, "scripts", "build.py"),
		filepath.Join(skillRoot, "scripts", "render_document.py"),
		filepath.Join(skillRoot, "assets", "fonts", "LICENSE-JetBrainsMono.txt"),
		filepath.Join(skillRoot, "requirements.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Kami Skill 缺少完整上游或 Nexus 适配资源 %s: %v", path, err)
		}
	}
}
