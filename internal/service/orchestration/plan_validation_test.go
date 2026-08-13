package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestValidatePlanDraftAcceptsSequentialRoomWork(t *testing.T) {
	draft := validPlanDraft()
	if err := ValidatePlanDraft(draft); err != nil {
		t.Fatalf("valid Plan rejected: %v", err)
	}
}

func TestPlanExecutionReturnsActionableRecoveryForEmptyWorkGraph(t *testing.T) {
	for _, test := range []struct {
		name  string
		draft PlanDraft
	}{
		{name: "empty array", draft: PlanDraft{}},
		{name: "placeholder object", draft: PlanDraft{Items: []PlanWorkItemDraft{{}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := testService(&fakeRepository{})
			result, err := service.PlanExecution(
				context.Background(),
				coordinatorActor(),
				PlanExecutionInput{
					CommandID:          "empty-workgraph",
					Objective:          "Deliver a verified report",
					CompletionCriteria: []string{"report accepted"},
					Draft:              test.draft,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != MutationRejected ||
				result.ReasonCode != ErrorCodePlanItemsEmpty ||
				len(result.NextActions) != 1 ||
				result.NextActions[0].Tool != "prepare_plan_execution" ||
				!strings.Contains(result.NextActions[0].Reason, "Plan Document") ||
				!strings.Contains(result.NextActions[0].Reason, "Work Item") {
				t.Fatalf("empty WorkGraph recovery = %#v", result)
			}
		})
	}
}

func TestValidatePlanDraftRejectsCycleAndUnknownDependency(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].DependsOn = []PlanDependencyDraft{{LogicalKey: "W2"}}
		err := ValidatePlanDraft(draft)
		assertDomainErrorCode(t, err, ErrorCodeDependencyCycle)
	})
	t.Run("unknown", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[1].DependsOn = []PlanDependencyDraft{{LogicalKey: "Missing"}}
		err := ValidatePlanDraft(draft)
		assertDomainErrorCode(t, err, ErrorCodeUnknownDependency)
	})
}

func TestValidatePlanDraftReportsSupportedWorkItemKinds(t *testing.T) {
	draft := validPlanDraft()
	draft.Items[0].Kind = protocol.WorkItemKind("task")
	err := ValidatePlanDraft(draft)
	assertDomainErrorCode(t, err, ErrorCodeInvalidInput)
	if !strings.Contains(err.Error(), "produce, review, verify, or integrate") {
		t.Fatalf("work item kind error is not actionable: %v", err)
	}
}

func TestValidatePlanDraftEnforcesTypedOutputScopeConflicts(t *testing.T) {
	t.Run("missing produce scope", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes = nil
		if err := ValidatePlanDraft(draft); err != nil {
			t.Fatalf("optional produce scope rejected: %v", err)
		}
	})
	t.Run("duplicate produce", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[1].DependsOn = nil
		draft.Items[1].OutputScopes = []protocol.WorkOutputScope{{
			Scope: "file:report/sources/facts.md",
			Mode:  protocol.WorkOutputScopeShared,
		}}
		err := ValidatePlanDraft(draft)
		assertDomainErrorCode(t, err, ErrorCodeOutputScopeConflict)
	})
	t.Run("shared review", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes[0].Mode = protocol.WorkOutputScopeShared
		draft.Items[1].Kind = protocol.WorkItemKindReview
		draft.Items[1].OutputScopes = []protocol.WorkOutputScope{{
			Scope: "file:report/sources/facts.md",
			Mode:  protocol.WorkOutputScopeShared,
		}}
		if err := ValidatePlanDraft(draft); err != nil {
			t.Fatalf("shared review output should be valid: %v", err)
		}
	})
	t.Run("untyped", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes[0].Scope = "report/sources"
		assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeInvalidInput)
	})
	t.Run("case-folded file", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[1].DependsOn = nil
		draft.Items[0].OutputScopes[0].Scope = "file:Report/Résumé.md"
		draft.Items[1].OutputScopes[0].Scope = "file:report/re\u0301sume\u0301.MD"
		assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeOutputScopeConflict)
	})
	t.Run("same item case-folded duplicate", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].OutputScopes = []protocol.WorkOutputScope{
			{Scope: "file:Report/Final.md"},
			{Scope: "file:report/final.MD"},
		}
		assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeInvalidInput)
	})
}

func TestValidatePlanDraftProjectionCollectionLimit(t *testing.T) {
	values := make([]string, protocol.ExecutionProjectionCollectionLimit)
	scopes := make([]protocol.WorkOutputScope, protocol.ExecutionProjectionCollectionLimit)
	for index := range values {
		values[index] = "value-" + string(rune('A'+index))
		scopes[index] = protocol.WorkOutputScope{
			Scope: "semantic:scope-" + string(rune('A'+index)),
			Mode:  protocol.WorkOutputScopeShared,
		}
	}

	atLimit := validPlanDraft()
	atLimit.Items[0].AcceptanceCriteria = append([]string(nil), values...)
	atLimit.Items[0].InputRefs = append([]string(nil), values...)
	atLimit.Items[0].OutputScopes = append([]protocol.WorkOutputScope(nil), scopes...)
	if err := ValidatePlanDraft(atLimit); err != nil {
		t.Fatalf("32-item collections rejected: %v", err)
	}

	overItemLimit := validPlanDraft()
	for len(overItemLimit.Items) <= protocol.ExecutionProjectionCollectionLimit {
		overItemLimit.Items = append(overItemLimit.Items, PlanWorkItemDraft{})
	}
	assertDomainErrorCode(
		t,
		ValidatePlanDraft(overItemLimit),
		ErrorCodeProjectionLimitExceeded,
	)

	tests := []struct {
		name   string
		mutate func(*PlanDraft)
	}{
		{name: "acceptance criteria", mutate: func(draft *PlanDraft) {
			draft.Items[0].AcceptanceCriteria = append(values, "overflow")
		}},
		{name: "input refs", mutate: func(draft *PlanDraft) {
			draft.Items[0].InputRefs = append(values, "overflow")
		}},
		{name: "output scopes", mutate: func(draft *PlanDraft) {
			draft.Items[0].OutputScopes = append(scopes, protocol.WorkOutputScope{Scope: "semantic:overflow"})
		}},
		{name: "direct dependencies", mutate: func(draft *PlanDraft) {
			draft.Items[2].DependsOn = make([]PlanDependencyDraft, protocol.ExecutionProjectionCollectionLimit+1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			draft := validPlanDraft()
			test.mutate(&draft)
			assertDomainErrorCode(t, ValidatePlanDraft(draft), ErrorCodeProjectionLimitExceeded)
		})
	}
}

func TestValidatePlanDraftAllowsAgentSelectedAcceptanceCriteriaAndTerminal(t *testing.T) {
	t.Run("criteria", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[0].AcceptanceCriteria = nil
		if err := ValidatePlanDraft(draft); err != nil {
			t.Fatalf("optional acceptance criteria rejected: %v", err)
		}
	})
	t.Run("terminal", func(t *testing.T) {
		draft := validPlanDraft()
		draft.Items[2].Terminal = false
		if err := ValidatePlanDraft(draft); err != nil {
			t.Fatalf("Plan without a terminal marker rejected: %v", err)
		}
	})
}

func TestNormalizeAndValidatePlanDraftReturnsCopyWithoutMutatingInput(t *testing.T) {
	draft := validPlanDraft()
	draft.RevisionReason = "  split evidence and analysis  "
	draft.Items[0].Subject = "  Collect evidence  "
	draft.Items[1].DependsOn[0].LogicalKey = " W1 "
	draft.Items[0].OutputScopes[0].Scope = " dir:report//sources/ "

	normalized, err := NormalizeAndValidatePlanDraft(draft)
	if err != nil {
		t.Fatalf("normalize valid Plan: %v", err)
	}
	if normalized.RevisionReason != "split evidence and analysis" ||
		normalized.Items[0].Subject != "Collect evidence" ||
		normalized.Items[1].DependsOn[0].LogicalKey != "W1" ||
		normalized.Items[0].OutputScopes[0].Scope != "dir:report/sources" {
		t.Fatalf("normalized Plan = %#v", normalized)
	}
	if draft.Items[1].DependsOn[0].LogicalKey != " W1 " ||
		draft.Items[0].OutputScopes[0].Scope != " dir:report//sources/ " {
		t.Fatalf("input draft was mutated: %#v", draft)
	}
}

func validPlanDraft() PlanDraft {
	return PlanDraft{Items: []PlanWorkItemDraft{
		{
			LogicalKey:         "W1",
			Kind:               protocol.WorkItemKindProduce,
			Subject:            "Collect evidence",
			Objective:          "Collect official facts",
			Deliverable:        "Source-backed fact table",
			AcceptanceCriteria: []string{"Every claim has a source"},
			Required:           true,
			OutputScopes: []protocol.WorkOutputScope{{
				Scope: "dir:report/sources",
				Mode:  protocol.WorkOutputScopeExclusive,
			}},
		},
		{
			LogicalKey:         "W2",
			Kind:               protocol.WorkItemKindProduce,
			Subject:            "Analyze",
			Objective:          "Compare accepted facts",
			Deliverable:        "Difference matrix",
			AcceptanceCriteria: []string{"Uses W1"},
			Required:           true,
			DependsOn:          []PlanDependencyDraft{{LogicalKey: "W1"}},
			OutputScopes: []protocol.WorkOutputScope{{
				Scope: "dir:report/analysis",
				Mode:  protocol.WorkOutputScopeExclusive,
			}},
		},
		{
			LogicalKey:         "W3",
			Kind:               protocol.WorkItemKindIntegrate,
			Subject:            "Integrate",
			Objective:          "Deliver the verified result",
			Deliverable:        "Final report",
			AcceptanceCriteria: []string{"Covers all requested dimensions"},
			Required:           true,
			Terminal:           true,
			DependsOn:          []PlanDependencyDraft{{LogicalKey: "W2"}},
		},
	}}
}

func assertDomainErrorCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	var domainErr *DomainError
	if !errors.As(err, &domainErr) {
		t.Fatalf("error = %v, want DomainError(%s)", err, want)
	}
	if domainErr.Code != want {
		t.Fatalf("error code = %s, want %s: %v", domainErr.Code, want, err)
	}
}
