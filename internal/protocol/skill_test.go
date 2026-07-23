package protocol

import "testing"

func TestExternalSkillReferenceRoundTrip(t *testing.T) {
	reference := BuildExternalSkillReference("demo-skill")
	if reference != "external:demo-skill" {
		t.Fatalf("外部 Skill 引用 = %q", reference)
	}
	if name, ok := ParseExternalSkillReference(reference); !ok || name != "demo-skill" {
		t.Fatalf("解析外部 Skill 引用 = %q, %v", name, ok)
	}
	if name, ok := ParseExternalSkillReference("External:demo-skill"); !ok || name != "demo-skill" {
		t.Fatalf("大小写不敏感解析失败: %q, %v", name, ok)
	}
}

func TestExternalSkillReferenceRejectsPathsAndNestedReferences(t *testing.T) {
	for _, value := range []string{"../skill", "nested/skill", "external:other"} {
		if reference := BuildExternalSkillReference(value); reference != "" {
			t.Fatalf("非法 Skill 名称 %q 被构造成 %q", value, reference)
		}
		if _, ok := ParseExternalSkillReference("external:" + value); ok {
			t.Fatalf("非法外部 Skill 引用 %q 被接受", value)
		}
	}
}
