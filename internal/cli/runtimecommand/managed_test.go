package runtimecommand

import (
	"slices"
	"testing"
)

func TestManagedSemanticSkillCatalogIsCompleteAndCallerOwned(t *testing.T) {
	names := ManagedSemanticSkillNames()
	for _, expected := range []string{GoalSkillName, ExecutionSkillName} {
		if !slices.Contains(names, expected) || !IsManagedSemanticSkillName(expected) {
			t.Fatalf("managed semantic Skill catalog is missing %q: %+v", expected, names)
		}
	}
	names[0] = "mutated-by-caller"
	if IsManagedSemanticSkillName("mutated-by-caller") || ManagedSemanticSkillNames()[0] == "mutated-by-caller" {
		t.Fatal("managed semantic Skill catalog leaked mutable package state")
	}
	if IsManagedSemanticSkillName("imagegen") {
		t.Fatal("unrelated Skill must not inherit managed semantic approval")
	}
}

func TestBindManagedSemanticSkillsCannotBeRemovedOrDisabled(t *testing.T) {
	bound, disabled := BindManagedSemanticSkills(
		[]string{"private-skill", "GOAL-MANAGER", "goal-manager"},
		[]string{"execution-orchestrator", "workspace-off"},
	)
	wantBound := []string{"private-skill", GoalSkillName, ExecutionSkillName}
	if !slices.Equal(bound, wantBound) {
		t.Fatalf("bound skills = %#v, want %#v", bound, wantBound)
	}
	if !slices.Equal(disabled, []string{"workspace-off"}) {
		t.Fatalf("disabled skills = %#v", disabled)
	}
}

func TestBindComputerUseSkillFollowsExplicitOwnerPreference(t *testing.T) {
	selected, disabled := BindComputerUseSkill(
		[]string{"private-skill", "COMPUTER-USE"},
		[]string{"computer-use", "workspace-off"},
		true,
	)
	if !slices.Equal(selected, []string{"private-skill", ComputerUseSkillName}) {
		t.Fatalf("enabled selected = %#v", selected)
	}
	if !slices.Equal(disabled, []string{"workspace-off"}) {
		t.Fatalf("enabled disabled = %#v", disabled)
	}

	selected, disabled = BindComputerUseSkill(selected, disabled, false)
	if !slices.Equal(selected, []string{"private-skill"}) {
		t.Fatalf("disabled selected = %#v", selected)
	}
	if !slices.Equal(disabled, []string{"workspace-off", ComputerUseSkillName}) {
		t.Fatalf("disabled disabled = %#v", disabled)
	}
}
