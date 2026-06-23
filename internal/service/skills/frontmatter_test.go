package skills

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToStringSliceReturnsEmptySlice(t *testing.T) {
	if result := toStringSlice(""); result == nil || len(result) != 0 {
		t.Fatalf("空字符串应返回空切片，实际: %#v", result)
	}
	if result := toStringSlice(nil); result == nil || len(result) != 0 {
		t.Fatalf("nil 应返回空切片，实际: %#v", result)
	}
}

func TestParseSkillFrontmatterWithoutTagsReturnsEmptySlice(t *testing.T) {
	parsed := parseSkillFrontmatter(`---
name: demo-skill
title: Demo Skill
description: no tags here
---

# Demo Skill
`, "demo-skill")
	if parsed.Tags == nil {
		t.Fatal("未声明 tags 时也必须返回空切片，不能是 nil")
	}
	if len(parsed.Tags) != 0 {
		t.Fatalf("未声明 tags 时应为空切片，实际: %#v", parsed.Tags)
	}
}

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

func TestSkillResponseSlicesMarshalAsEmptyArray(t *testing.T) {
	info := Info{
		Name: "demo-skill",
		Tags: firstNonEmptySlice(nil),
	}
	payload, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("序列化技能信息失败: %v", err)
	}
	if string(payload) == "" || !strings.Contains(string(payload), `"tags":[]`) {
		t.Fatalf("tags 未按协议序列化为空数组: %s", string(payload))
	}
}
