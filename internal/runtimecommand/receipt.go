// INPUT: broker 已执行的 Goal/Execution operation、typed result 与调用时 exact Goal binding。
// OUTPUT: DM/Room runtime 可按 sequence 单调消费的 host-side mutation receipt。
// POS: CLI transport 与 Goal usage/continuation/完成收据之间的事实桥；禁止解析 shell stdout 推断。
package runtimecommand

import (
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	GoalOperationCreate       = "create_goal"
	GoalOperationRetarget     = "retarget_goal"
	GoalOperationAudit        = "audit_objective_alignment"
	GoalOperationUpdate       = "update_goal"
	ExecutionOperationPlan    = "plan_execution"
	ExecutionOperationPromote = "promote_execution_to_goal"
)

type Receipt struct {
	Sequence         uint64 `json:"sequence"`
	RequestID        string `json:"request_id"`
	Domain           string `json:"domain"`
	Operation        string `json:"operation"`
	Outcome          string `json:"outcome,omitempty"`
	Message          string `json:"message,omitempty"`
	ReasonCode       string `json:"reason_code,omitempty"`
	GoalID           string `json:"goal_id,omitempty"`
	GoalStatus       string `json:"goal_status,omitempty"`
	ExecutionID      string `json:"execution_id,omitempty"`
	WorkItemID       string `json:"work_item_id,omitempty"`
	AssignmentID     string `json:"assignment_id,omitempty"`
	AttemptID        string `json:"attempt_id,omitempty"`
	SnapshotRevision int64  `json:"snapshot_revision,omitempty"`
	GoalBound        bool   `json:"goal_bound,omitempty"`
}

type ReceiptState struct {
	mu       sync.RWMutex
	sequence uint64
	receipts []Receipt
}

func NewReceiptState() *ReceiptState { return &ReceiptState{} }

func (s *ReceiptState) Record(receipt Receipt) Receipt {
	if s == nil {
		return Receipt{}
	}
	receipt.Domain = strings.TrimSpace(receipt.Domain)
	receipt.RequestID = strings.TrimSpace(receipt.RequestID)
	receipt.Operation = strings.TrimSpace(receipt.Operation)
	receipt.Outcome = strings.TrimSpace(receipt.Outcome)
	receipt.Message = strings.TrimSpace(receipt.Message)
	receipt.ReasonCode = strings.TrimSpace(receipt.ReasonCode)
	receipt.GoalID = strings.TrimSpace(receipt.GoalID)
	receipt.GoalStatus = strings.TrimSpace(receipt.GoalStatus)
	receipt.ExecutionID = strings.TrimSpace(receipt.ExecutionID)
	receipt.WorkItemID = strings.TrimSpace(receipt.WorkItemID)
	receipt.AssignmentID = strings.TrimSpace(receipt.AssignmentID)
	receipt.AttemptID = strings.TrimSpace(receipt.AttemptID)
	s.mu.Lock()
	s.sequence++
	receipt.Sequence = s.sequence
	s.receipts = append(s.receipts, receipt)
	s.mu.Unlock()
	return receipt
}

func (s *ReceiptState) Since(sequence uint64) ([]Receipt, uint64) {
	if s == nil {
		return nil, sequence
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sequence >= s.sequence {
		return nil, s.sequence
	}
	start := sequence
	if start > uint64(len(s.receipts)) {
		start = uint64(len(s.receipts))
	}
	result := append([]Receipt(nil), s.receipts[int(start):]...)
	return result, s.sequence
}

// Applied reports whether the broker observed a successful semantic mutation.
// Empty outcomes are not accepted: every mutation adapter must return an exact
// applied/no-op/rejected/superseded outcome before Goal accounting consumes it.
func (r Receipt) Applied() bool {
	return strings.TrimSpace(r.Outcome) == string(protocol.MutationResultApplied)
}

// CountsAsGoalProgress preserves the Goal continuation contract without
// depending on Provider tool names or parsing CLI stdout.
func (r Receipt) CountsAsGoalProgress() bool {
	if !r.Applied() {
		return false
	}
	switch strings.TrimSpace(r.Domain) {
	case DomainGoal:
		switch strings.TrimSpace(r.Operation) {
		case GoalOperationCreate, GoalOperationRetarget, GoalOperationAudit, GoalOperationUpdate:
			return true
		}
	case DomainExecution:
		if !r.GoalBound {
			return false
		}
		switch strings.TrimSpace(r.Operation) {
		case ExecutionOperationPlan, "abandon_execution", "assign_work", "submit_work",
			"review_work", "block_work", "resume_work", "take_over_work",
			"audit_execution_alignment":
			return true
		}
	}
	return false
}

func HasGoalProgress(receipts []Receipt) bool {
	for _, receipt := range receipts {
		if receipt.CountsAsGoalProgress() {
			return true
		}
	}
	return false
}

func HasAppliedOperation(receipts []Receipt, domain, operation string) bool {
	for _, receipt := range receipts {
		if receipt.Applied() && receipt.Domain == domain && receipt.Operation == operation {
			return true
		}
	}
	return false
}

func HasDomain(receipts []Receipt, domain string) bool {
	domain = strings.TrimSpace(domain)
	for _, receipt := range receipts {
		if receipt.Domain == domain {
			return true
		}
	}
	return false
}

// SuccessfulGoalCompletionID returns the exact Goal identity from an applied
// update_goal completion receipt. Conflicting host binding and receipt identity
// fail closed.
func SuccessfulGoalCompletionID(receipts []Receipt, boundGoalID string) string {
	boundGoalID = strings.TrimSpace(boundGoalID)
	for _, receipt := range receipts {
		if !receipt.Applied() || receipt.Domain != DomainGoal ||
			receipt.Operation != GoalOperationUpdate ||
			strings.TrimSpace(receipt.GoalStatus) != string(protocol.GoalStatusComplete) {
			continue
		}
		goalID := strings.TrimSpace(receipt.GoalID)
		if goalID == "" {
			goalID = boundGoalID
		}
		if goalID == "" || boundGoalID != "" && goalID != boundGoalID {
			continue
		}
		return goalID
	}
	return ""
}
