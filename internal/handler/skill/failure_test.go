package skill

import (
	"errors"
	"net/http"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProjectSkillMutationFailurePreservesOutcomeEvidence(t *testing.T) {
	tests := []struct {
		name           string
		needsReconcile bool
		applied        bool
		wantEffect     protocol.FailureEffect
		wantCode       string
	}{
		{
			name:       "pre-commit failure",
			wantEffect: protocol.FailureEffectNotApplied,
			wantCode:   "skill.import_git_failed",
		},
		{
			name:           "commit outcome unknown",
			needsReconcile: true,
			wantEffect:     protocol.FailureEffectUnknown,
			wantCode:       "skill.import_git_failed.outcome_unknown",
		},
		{
			name:           "committed with reconcile debt",
			needsReconcile: true,
			applied:        true,
			wantEffect:     protocol.FailureEffectCommitted,
			wantCode:       "skill.import_git_failed.reconcile_required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, spec := projectSkillMutationFailure(
				errors.New("internal detail"),
				"skill.import_git_failed",
				http.StatusBadRequest,
				protocol.FailureCategoryValidation,
				"Git 技能没有导入",
				test.needsReconcile,
				test.applied,
			)
			if status != http.StatusBadRequest {
				t.Fatalf("status=%d, want %d", status, http.StatusBadRequest)
			}
			if spec.Effect != test.wantEffect || spec.Code != test.wantCode {
				t.Fatalf("spec=%+v, want effect=%s code=%s", spec, test.wantEffect, test.wantCode)
			}
			if spec.Detail == "internal detail" {
				t.Fatalf("internal cause must not become user detail: %+v", spec)
			}
			if test.needsReconcile && (spec.Resolution == nil || spec.Resolution.Action != "skill.refresh_catalog") {
				t.Fatalf("reconcile resolution=%+v", spec.Resolution)
			}
			if !test.needsReconcile && spec.Resolution != nil {
				t.Fatalf("pre-commit failure must not invent recovery action: %+v", spec.Resolution)
			}
		})
	}
}
