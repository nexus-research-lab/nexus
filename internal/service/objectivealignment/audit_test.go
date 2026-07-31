package objectivealignment

import (
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAuditNormalizesAuthoritativeCriterionOrderAndThreeStateDecision(t *testing.T) {
	target := Target{
		Objective: "Ship the report",
		Criteria:  []string{"tests pass", "report delivered"},
	}
	report, err := Audit(target, protocol.ObjectiveAlignmentReport{
		Decision: protocol.ObjectiveAlignmentNotAligned,
		CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{
			{
				Criterion: "report delivered",
				Status:    protocol.ObjectiveAlignmentCriterionUnsatisfied,
				Gap:       "the report file is missing",
			},
			{
				Criterion: "tests pass",
				Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
				Evidence: []protocol.ObjectiveAlignmentEvidence{{
					Ref:   "command:make-check",
					Claim: "all required checks passed",
				}},
			},
		},
		Summary: "Tests pass, but delivery is missing.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.CriteriaResults[0].Criterion != "tests pass" ||
		report.CriteriaResults[1].Criterion != "report delivered" {
		t.Fatalf("criterion order = %+v", report.CriteriaResults)
	}
}

func TestAuditRejectsAlignedClaimWithoutCompleteEvidence(t *testing.T) {
	target := Target{
		Objective: "Ship the report",
		Criteria:  []string{"tests pass", "report delivered"},
	}
	_, err := RequireAligned(target, protocol.ObjectiveAlignmentReport{
		Decision: protocol.ObjectiveAlignmentAligned,
		CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
			Criterion: "tests pass",
			Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
			Evidence: []protocol.ObjectiveAlignmentEvidence{{
				Ref:   "command:make-check",
				Claim: "checks passed",
			}},
		}},
		Summary: "Complete.",
	})
	if !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("RequireAligned error = %v, want ErrInvalidReport", err)
	}
}

func TestAuditKeepsInconclusiveDistinctFromConfirmedGap(t *testing.T) {
	target := Target{Objective: "Verify deployment"}
	report, err := Audit(target, protocol.ObjectiveAlignmentReport{
		Decision: protocol.ObjectiveAlignmentInconclusive,
		CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
			Criterion: "Verify deployment",
			Status:    protocol.ObjectiveAlignmentCriterionInconclusive,
			Gap:       "production status is unavailable",
		}},
		Summary: "Deployment cannot yet be verified.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != protocol.ObjectiveAlignmentInconclusive {
		t.Fatalf("decision = %q", report.Decision)
	}
}
