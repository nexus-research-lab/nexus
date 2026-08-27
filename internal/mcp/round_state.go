// INPUT: physical round 的可信宿主上下文、命令尝试与 typed mutation result。
// OUTPUT: 内置 MCP 共享上下文、失败计数及可按 sequence 单调消费的 mutation receipt。
// POS: runtime 与结构化 command、Goal usage/continuation 之间的宿主事实桥；禁止从模型文本推断。
package mcp

import (
	"slices"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

// RoundContext 是一个 physical round 的完整内置 MCP 上下文。
type RoundContext struct {
	SessionKey         string
	RoundID            string
	SourceContextType  string
	SourceContextID    string
	SourceContextLabel string
	CommandContext     runtimectx.RuntimeCommandContext
	CommandReceipts    *CommandReceiptState
	CommandAttempts    *CommandAttemptState
}

// CommandAttemptState 保存一次 physical round 内的有界传输失败计数。
type CommandAttemptState struct {
	mu     sync.Mutex
	counts map[string]uint32
}

func NewCommandAttemptState() *CommandAttemptState {
	return &CommandAttemptState{counts: make(map[string]uint32)}
}

func (s *CommandAttemptState) Increment(key string) uint32 {
	if s == nil {
		return 1
	}
	key = strings.TrimSpace(key)
	s.mu.Lock()
	s.counts[key]++
	value := s.counts[key]
	s.mu.Unlock()
	return value
}

func (s *CommandAttemptState) Reset(key string) {
	if s == nil {
		return
	}
	key = strings.TrimSpace(key)
	s.mu.Lock()
	delete(s.counts, key)
	s.mu.Unlock()
}

const (
	CommandDomainGoal         = "goal"
	CommandDomainExecution    = "execution"
	GoalOperationCreate       = "create_goal"
	GoalOperationRetarget     = "retarget_goal"
	GoalOperationAudit        = "audit_objective_alignment"
	GoalOperationUpdate       = "update_goal"
	ExecutionOperationPlan    = "plan_execution"
	ExecutionOperationPromote = "promote_execution_to_goal"
)

type CommandReceipt struct {
	Sequence         uint64   `json:"sequence"`
	RequestID        string   `json:"request_id"`
	Domain           string   `json:"domain"`
	Operation        string   `json:"operation"`
	Outcome          string   `json:"outcome,omitempty"`
	Message          string   `json:"message,omitempty"`
	ReasonCode       string   `json:"reason_code,omitempty"`
	GoalID           string   `json:"goal_id,omitempty"`
	GoalStatus       string   `json:"goal_status,omitempty"`
	ExecutionID      string   `json:"execution_id,omitempty"`
	WorkItemID       string   `json:"work_item_id,omitempty"`
	AssignmentID     string   `json:"assignment_id,omitempty"`
	AttemptID        string   `json:"attempt_id,omitempty"`
	Changed          []string `json:"changed,omitempty"`
	SnapshotRevision int64    `json:"snapshot_revision,omitempty"`
	GoalBound        bool     `json:"goal_bound,omitempty"`
}

type CommandReceiptState struct {
	mu       sync.RWMutex
	sequence uint64
	receipts []CommandReceipt
}

func NewCommandReceiptState() *CommandReceiptState { return &CommandReceiptState{} }

func (s *CommandReceiptState) Record(receipt CommandReceipt) CommandReceipt {
	if s == nil {
		return CommandReceipt{}
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
	receipt.Changed = slices.DeleteFunc(slices.Clone(receipt.Changed), func(value string) bool {
		return strings.TrimSpace(value) == ""
	})
	for index := range receipt.Changed {
		receipt.Changed[index] = strings.TrimSpace(receipt.Changed[index])
	}
	s.mu.Lock()
	s.sequence++
	receipt.Sequence = s.sequence
	s.receipts = append(s.receipts, receipt)
	s.mu.Unlock()
	return receipt
}

func (s *CommandReceiptState) Since(sequence uint64) ([]CommandReceipt, uint64) {
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
	result := append([]CommandReceipt(nil), s.receipts[int(start):]...)
	return result, s.sequence
}

// Applied 判断宿主是否观察到成功的语义变更。
// 空结果不计成功：Goal 结算消费前，每个变更适配器都必须返回精确结果。
func (r CommandReceipt) Applied() bool {
	return strings.TrimSpace(r.Outcome) == string(protocol.MutationResultApplied)
}

// CountsAsGoalProgress 在不依赖 Provider 工具名或解析文本输出的情况下保持 Goal continuation 契约。
func (r CommandReceipt) CountsAsGoalProgress() bool {
	if !r.Applied() {
		return false
	}
	switch strings.TrimSpace(r.Domain) {
	case CommandDomainGoal:
		switch strings.TrimSpace(r.Operation) {
		case GoalOperationCreate, GoalOperationRetarget, GoalOperationAudit, GoalOperationUpdate:
			return true
		}
	case CommandDomainExecution:
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

func HasGoalProgress(receipts []CommandReceipt) bool {
	for _, receipt := range receipts {
		if receipt.CountsAsGoalProgress() {
			return true
		}
	}
	return false
}

func HasAppliedOperation(receipts []CommandReceipt, domain, operation string) bool {
	for _, receipt := range receipts {
		if receipt.Applied() && receipt.Domain == domain && receipt.Operation == operation {
			return true
		}
	}
	return false
}

func HasDomain(receipts []CommandReceipt, domain string) bool {
	domain = strings.TrimSpace(domain)
	for _, receipt := range receipts {
		if receipt.Domain == domain {
			return true
		}
	}
	return false
}

// SuccessfulGoalCompletionID 从已应用的 update_goal 完成回执返回精确 Goal 身份。
// 宿主绑定与回执身份冲突时按失败处理。
func SuccessfulGoalCompletionID(receipts []CommandReceipt, boundGoalID string) string {
	boundGoalID = strings.TrimSpace(boundGoalID)
	for _, receipt := range receipts {
		if !receipt.Applied() || receipt.Domain != CommandDomainGoal ||
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
