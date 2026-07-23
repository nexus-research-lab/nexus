package skills

import (
	"slices"
	"testing"
)

func TestEnsureExternalSkillReferenceCanonicalizesLegacyValues(t *testing.T) {
	want := []string{"imagegen", "external:demo-skill"}
	for _, selected := range [][]string{
		{"imagegen", "demo-skill"},
		{"imagegen", "External:demo-skill"},
		{"imagegen", "external:demo-skill", "demo-skill"},
	} {
		got, changed := ensureExternalSkillReference(selected, "demo-skill")
		if !changed || !slices.Equal(got, want) {
			t.Fatalf("规范化 %#v = %#v, changed=%v, want %#v", selected, got, changed, want)
		}
	}
}

func TestEnsureExternalSkillReferenceKeepsCanonicalValue(t *testing.T) {
	selected := []string{"imagegen", "external:demo-skill"}
	got, changed := ensureExternalSkillReference(selected, "demo-skill")
	if changed || !slices.Equal(got, selected) {
		t.Fatalf("canonical 引用不应变化: %#v, changed=%v", got, changed)
	}
}
