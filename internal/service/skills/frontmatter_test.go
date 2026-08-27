package skills

import (
	"strings"
	"testing"
)

func TestParseSkillFrontmatterBlockDescription(t *testing.T) {
	parsed := parseSkillFrontmatter(`---
name: chronicle
description: |
  Allows you to view the user's screen as well as several hours of history.

  Use this skill when recent screen context is needed.
tags: [screen, context]
---

# Chronicle
`, "chronicle")
	if parsed.Description == "" || strings.Contains(parsed.Description, "|") {
		t.Fatalf("多行 description 解析不正确: %q", parsed.Description)
	}
	if !strings.Contains(parsed.Description, "recent screen context") {
		t.Fatalf("多行 description 内容丢失: %q", parsed.Description)
	}
	if len(parsed.Tags) != 2 || parsed.Tags[0] != "screen" || parsed.Tags[1] != "context" {
		t.Fatalf("block scalar 后续字段解析不正确: %#v", parsed.Tags)
	}
}

func TestParseSkillFrontmatterReadsNexusMetadataContainer(t *testing.T) {
	parsed := parseSkillFrontmatter(`---
name: office-demo
description: Office demo
metadata:
  title: Office Demo
  version: 2.0.0
  category_key: content-docs
  category_name: 内容与文档
  recommendation: 适合创建和校验办公文件。
  tags:
    - office
    - document-editing
---

# Office Demo
`, "office-demo")
	if parsed.Title != "Office Demo" || parsed.Version != "2.0.0" {
		t.Fatalf("metadata 标题或版本解析不正确: %+v", parsed)
	}
	if parsed.CategoryKey != "content-docs" || parsed.CategoryName != "内容与文档" {
		t.Fatalf("metadata 分类解析不正确: %+v", parsed)
	}
	if parsed.Recommendation != "适合创建和校验办公文件。" {
		t.Fatalf("metadata 推荐语解析不正确: %q", parsed.Recommendation)
	}
	if len(parsed.Tags) != 2 || parsed.Tags[0] != "office" || parsed.Tags[1] != "document-editing" {
		t.Fatalf("metadata 标签解析不正确: %#v", parsed.Tags)
	}
}

func TestStripFrontmatterReturnsSkillBody(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "yaml frontmatter",
			content: "---\nname: demo\n---\n\n# Demo\n",
			want:    "# Demo\n",
		},
		{
			name:    "bom",
			content: "\ufeff---\nname: demo\n---\nbody",
			want:    "body",
		},
		{
			name:    "plain markdown",
			content: "# Demo",
			want:    "# Demo",
		},
		{
			name:    "unterminated frontmatter",
			content: "---\nname: demo\n# Demo",
			want:    "---\nname: demo\n# Demo",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := StripFrontmatter(test.content); got != test.want {
				t.Fatalf("正文投影 = %q，期望 %q", got, test.want)
			}
		})
	}
}
