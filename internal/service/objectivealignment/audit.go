// INPUT: 服务端权威 objective/criteria 与提交方给出的逐项 evidence report。
// OUTPUT: 规范化三态报告、稳定 target fingerprint 或明确的契约错误。
// POS: Goal completion 与 loop guard 之间无状态、无生命周期依赖的共享判定内核。
package objectivealignment

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var (
	ErrInvalidTarget = errors.New("objective alignment target is invalid")
	ErrInvalidReport = errors.New("objective alignment report is invalid")
	ErrNotAligned    = errors.New("objective is not aligned")
)

// Target 是一次审计不可由判定方改写的权威完成边界。
type Target struct {
	Objective string
	Criteria  []string
}

// NormalizeTarget 生成稳定、非空且无重复的完成边界。
func NormalizeTarget(target Target) (Target, error) {
	target.Objective = strings.TrimSpace(target.Objective)
	if target.Objective == "" {
		return Target{}, fmt.Errorf("%w: objective is required", ErrInvalidTarget)
	}
	if len(target.Criteria) > protocol.ObjectiveAlignmentCollectionLimit {
		return Target{}, fmt.Errorf(
			"%w: criteria exceed limit %d",
			ErrInvalidTarget,
			protocol.ObjectiveAlignmentCollectionLimit,
		)
	}
	criteria := make([]string, 0, len(target.Criteria))
	seen := make(map[string]struct{}, len(target.Criteria))
	for _, criterion := range target.Criteria {
		criterion = strings.TrimSpace(criterion)
		if criterion == "" {
			continue
		}
		if _, duplicate := seen[criterion]; duplicate {
			return Target{}, fmt.Errorf(
				"%w: criterion %q appears more than once",
				ErrInvalidTarget,
				criterion,
			)
		}
		seen[criterion] = struct{}{}
		criteria = append(criteria, criterion)
	}
	if len(criteria) == 0 {
		criteria = []string{target.Objective}
	}
	target.Criteria = criteria
	return target, nil
}

// Audit 验证逐项覆盖、证据、缺口和整体 decision 是否彼此一致。
func Audit(
	target Target,
	report protocol.ObjectiveAlignmentReport,
) (protocol.ObjectiveAlignmentReport, error) {
	target, err := NormalizeTarget(target)
	if err != nil {
		return protocol.ObjectiveAlignmentReport{}, err
	}
	if len(report.CriteriaResults) > protocol.ObjectiveAlignmentCollectionLimit {
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"%w: criterion results exceed limit %d",
			ErrInvalidReport,
			protocol.ObjectiveAlignmentCollectionLimit,
		)
	}
	byCriterion := make(
		map[string]protocol.ObjectiveAlignmentCriterionResult,
		len(report.CriteriaResults),
	)
	for _, result := range report.CriteriaResults {
		normalized, normalizeErr := normalizeCriterionResult(result)
		if normalizeErr != nil {
			return protocol.ObjectiveAlignmentReport{}, normalizeErr
		}
		if _, duplicate := byCriterion[normalized.Criterion]; duplicate {
			return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
				"%w: criterion %q appears more than once",
				ErrInvalidReport,
				normalized.Criterion,
			)
		}
		byCriterion[normalized.Criterion] = normalized
	}

	ordered := make(
		[]protocol.ObjectiveAlignmentCriterionResult,
		0,
		len(target.Criteria),
	)
	hasUnsatisfied := false
	hasInconclusive := false
	for _, criterion := range target.Criteria {
		result, exists := byCriterion[criterion]
		if !exists {
			return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
				"%w: criterion %q has no result",
				ErrInvalidReport,
				criterion,
			)
		}
		delete(byCriterion, criterion)
		switch result.Status {
		case protocol.ObjectiveAlignmentCriterionSatisfied:
			if len(result.Evidence) == 0 {
				return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
					"%w: satisfied criterion %q requires evidence",
					ErrInvalidReport,
					criterion,
				)
			}
		case protocol.ObjectiveAlignmentCriterionUnsatisfied:
			hasUnsatisfied = true
			if result.Gap == "" {
				return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
					"%w: unsatisfied criterion %q requires a gap",
					ErrInvalidReport,
					criterion,
				)
			}
		case protocol.ObjectiveAlignmentCriterionInconclusive:
			hasInconclusive = true
			if result.Gap == "" {
				return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
					"%w: inconclusive criterion %q requires the missing or conflicting evidence",
					ErrInvalidReport,
					criterion,
				)
			}
		default:
			return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
				"%w: criterion %q has unknown status %q",
				ErrInvalidReport,
				criterion,
				result.Status,
			)
		}
		ordered = append(ordered, result)
	}
	for criterion := range byCriterion {
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"%w: criterion %q is not part of the authoritative target",
			ErrInvalidReport,
			criterion,
		)
	}

	expectedDecision := protocol.ObjectiveAlignmentAligned
	switch {
	case hasUnsatisfied:
		expectedDecision = protocol.ObjectiveAlignmentNotAligned
	case hasInconclusive:
		expectedDecision = protocol.ObjectiveAlignmentInconclusive
	}
	if report.Decision != expectedDecision {
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"%w: decision %q does not match criterion aggregate %q",
			ErrInvalidReport,
			report.Decision,
			expectedDecision,
		)
	}
	report.CriteriaResults = ordered
	report.Summary = strings.TrimSpace(report.Summary)
	if report.Summary == "" {
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"%w: summary is required",
			ErrInvalidReport,
		)
	}
	return report, nil
}

// RequireAligned 只接受已经通过完整 Audit 的 aligned 报告。
func RequireAligned(
	target Target,
	report protocol.ObjectiveAlignmentReport,
) (protocol.ObjectiveAlignmentReport, error) {
	normalized, err := Audit(target, report)
	if err != nil {
		return protocol.ObjectiveAlignmentReport{}, err
	}
	if normalized.Decision != protocol.ObjectiveAlignmentAligned {
		return protocol.ObjectiveAlignmentReport{}, fmt.Errorf(
			"%w: decision is %s",
			ErrNotAligned,
			normalized.Decision,
		)
	}
	return normalized, nil
}

// Fingerprint 把 objective 与有序 criteria 固定为生命周期可校验的目标身份。
func Fingerprint(target Target) (string, error) {
	normalized, err := NormalizeTarget(target)
	if err != nil {
		return "", err
	}
	sum := sha256.New()
	_, _ = sum.Write([]byte(normalized.Objective))
	for _, criterion := range normalized.Criteria {
		_, _ = sum.Write([]byte{0})
		_, _ = sum.Write([]byte(criterion))
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// ReportsEqual 用于同 round 工具重试的语义幂等判断。
func ReportsEqual(
	left protocol.ObjectiveAlignmentReport,
	right protocol.ObjectiveAlignmentReport,
) bool {
	return reflect.DeepEqual(left, right)
}

func normalizeCriterionResult(
	result protocol.ObjectiveAlignmentCriterionResult,
) (protocol.ObjectiveAlignmentCriterionResult, error) {
	result.Criterion = strings.TrimSpace(result.Criterion)
	result.Gap = strings.TrimSpace(result.Gap)
	if result.Criterion == "" {
		return protocol.ObjectiveAlignmentCriterionResult{}, fmt.Errorf(
			"%w: criterion is required",
			ErrInvalidReport,
		)
	}
	if len(result.Evidence) > protocol.ObjectiveAlignmentCollectionLimit {
		return protocol.ObjectiveAlignmentCriterionResult{}, fmt.Errorf(
			"%w: evidence for criterion %q exceeds limit %d",
			ErrInvalidReport,
			result.Criterion,
			protocol.ObjectiveAlignmentCollectionLimit,
		)
	}
	evidence := make([]protocol.ObjectiveAlignmentEvidence, 0, len(result.Evidence))
	seen := make(map[string]struct{}, len(result.Evidence))
	for _, item := range result.Evidence {
		item.Ref = strings.TrimSpace(item.Ref)
		item.Claim = strings.TrimSpace(item.Claim)
		if item.Ref == "" || item.Claim == "" {
			return protocol.ObjectiveAlignmentCriterionResult{}, fmt.Errorf(
				"%w: evidence for criterion %q requires ref and claim",
				ErrInvalidReport,
				result.Criterion,
			)
		}
		key := item.Ref + "\x00" + item.Claim
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		evidence = append(evidence, item)
	}
	result.Evidence = evidence
	return result, nil
}
