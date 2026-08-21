// INPUT: 已按 Goal/document/current Execution 选择 canonical root boundary 并完成语义校验、但尚未成为权威 Execution/Plan 的完整 WorkGraph proposal。
// OUTPUT: 跨 round 可恢复的 sealed proposal、materialization receipt 与 Goal confirmation 状态。
// POS: Provider/Plan Mode 与 Execution Orchestration 权威事务之间的非权威持久化协议；持久 proposal 不保留非权威 Goal objective 转述。
package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

const (
	// ExecutionPlanProposalDocumentVersion 是 canonical proposal document 的当前版本。
	ExecutionPlanProposalDocumentVersion = 1
)

// ExecutionPlanProposalOperation 描述 sealed proposal 的权威 materialization 目标。
type ExecutionPlanProposalOperation string

const (
	ExecutionPlanProposalCreate  ExecutionPlanProposalOperation = "create"
	ExecutionPlanProposalReplan  ExecutionPlanProposalOperation = "replan"
	ExecutionPlanProposalReplace ExecutionPlanProposalOperation = "replace"
)

// ExecutionPlanProposalStatus 描述非权威 proposal 自身的恢复生命周期。
type ExecutionPlanProposalStatus string

const (
	ExecutionPlanProposalStatusSealed        ExecutionPlanProposalStatus = "sealed"
	ExecutionPlanProposalStatusMaterializing ExecutionPlanProposalStatus = "materializing"
	ExecutionPlanProposalStatusMaterialized  ExecutionPlanProposalStatus = "materialized"
	ExecutionPlanProposalStatusBlocked       ExecutionPlanProposalStatus = "blocked"
	ExecutionPlanProposalStatusDiscarded     ExecutionPlanProposalStatus = "discarded"
)

// ExecutionPlanProposalConfirmationState 表示 materialized Goal binding 的确认状态。
type ExecutionPlanProposalConfirmationState string

const (
	ExecutionPlanProposalConfirmationNone      ExecutionPlanProposalConfirmationState = "none"
	ExecutionPlanProposalConfirmationPending   ExecutionPlanProposalConfirmationState = "pending"
	ExecutionPlanProposalConfirmationConfirmed ExecutionPlanProposalConfirmationState = "confirmed"
)

// ExecutionPlanProposalDocument 是 parser、持久层和 materializer 共享的 canonical 文档。
// Parser 可暂时产出缺省 boundary；service 必须按 operation/authority 补全后才能 digest 与持久化。
// Items 的顺序就是未来 Plan membership 的 position；调用方必须在计算 digest 前完成
// string normalization 以及 dependency/output scope 的稳定排序。
type ExecutionPlanProposalDocument struct {
	Version             int                            `json:"nexus_plan"`
	Operation           ExecutionPlanProposalOperation `json:"operation"`
	Objective           string                         `json:"objective"`
	CompletionCriteria  []string                       `json:"completion_criteria"`
	RevisionReason      string                         `json:"revision_reason,omitempty"`
	SupersedeActiveWork bool                           `json:"supersede_active_work,omitempty"`
	ReplacementReason   string                         `json:"replacement_reason,omitempty"`
	Items               []ExecutionPlanProposalItem    `json:"items"`
}

// ExecutionPlanProposalItem 使用 logical key 表达未 materialize WorkGraph 中的稳定引用。
type ExecutionPlanProposalItem struct {
	LogicalKey         string                            `json:"logical_key"`
	ExistingWorkItemID string                            `json:"existing_work_item_id,omitempty"`
	Kind               WorkItemKind                      `json:"kind"`
	Subject            string                            `json:"subject"`
	Objective          string                            `json:"objective"`
	Deliverable        string                            `json:"deliverable"`
	AcceptanceCriteria []string                          `json:"acceptance_criteria"`
	Required           bool                              `json:"required"`
	Terminal           bool                              `json:"terminal"`
	ParentLogicalKey   string                            `json:"parent_logical_key,omitempty"`
	DependsOn          []ExecutionPlanProposalDependency `json:"depends_on,omitempty"`
	InputRefs          []string                          `json:"input_refs,omitempty"`
	OutputScopes       []WorkOutputScope                 `json:"output_scopes,omitempty"`
}

// ExecutionPlanProposalDependency 是 proposal 内的 typed logical-key edge。
type ExecutionPlanProposalDependency struct {
	LogicalKey string             `json:"logical_key"`
	Kind       WorkDependencyKind `json:"kind"`
}

// ExecutionPlanProposal 是 sealed immutable document 及其可恢复 materialization receipt。
// Proposal identity 本身不是 capability；每次读取和 mutation 都必须重新验证 owner、
// session、scope 与 coordinator。
type ExecutionPlanProposal struct {
	ID                 string             `json:"id"`
	OwnerUserID        string             `json:"owner_user_id"`
	SessionKey         string             `json:"session_key"`
	ScopeKind          ExecutionScopeKind `json:"scope_kind"`
	RoomID             string             `json:"room_id,omitempty"`
	ConversationID     string             `json:"conversation_id,omitempty"`
	CoordinatorAgentID string             `json:"coordinator_agent_id"`
	RootRoundID        string             `json:"root_round_id,omitempty"`
	RuntimeRoundID     string             `json:"runtime_round_id,omitempty"`
	AgentRoundID       string             `json:"agent_round_id,omitempty"`

	TargetExecutionID      string `json:"target_execution_id,omitempty"`
	TargetExecutionVersion int64  `json:"target_execution_version,omitempty"`
	BasePlanID             string `json:"base_plan_id,omitempty"`

	GoalID                string               `json:"goal_id,omitempty"`
	GoalObjectiveRevision int64                `json:"goal_objective_revision,omitempty"`
	GoalActivationOrigin  GoalActivationOrigin `json:"goal_activation_origin,omitempty"`
	GoalActivationReason  GoalActivationReason `json:"goal_activation_reason,omitempty"`
	// GoalReservedExecutionID 是 explicit Goal 创建或 Goal transition 在
	// proposal seal 之前已经持久化的 successor identity。旧显式 Goal 可从
	// server-owned command 确定性恢复；它仍属于 immutable exact fence，不是
	// materializer 可以另行生成或改写的运行态 receipt。
	GoalReservedExecutionID string `json:"goal_reserved_execution_id,omitempty"`
	ReplacesExecutionID     string `json:"replaces_execution_id,omitempty"`

	Document      ExecutionPlanProposalDocument `json:"document"`
	ContentDigest string                        `json:"content_digest"`
	Status        ExecutionPlanProposalStatus   `json:"status"`
	Version       int64                         `json:"version"`

	ReservedExecutionID      string `json:"reserved_execution_id,omitempty"`
	MaterializationCommandID string `json:"materialization_command_id,omitempty"`
	MaterializedExecutionID  string `json:"materialized_execution_id,omitempty"`
	MaterializedPlanID       string `json:"materialized_plan_id,omitempty"`

	ConfirmationState ExecutionPlanProposalConfirmationState `json:"confirmation_state"`
	AttemptCount      int                                    `json:"attempt_count"`
	NextAttemptAt     *time.Time                             `json:"next_attempt_at,omitempty"`
	LastError         string                                 `json:"last_error,omitempty"`
	CreatedAt         time.Time                              `json:"created_at"`
	UpdatedAt         time.Time                              `json:"updated_at"`
	MaterializedAt    *time.Time                             `json:"materialized_at,omitempty"`
}

// Normalized 返回去除首尾空白的 proposal 副本。清洗只发生在 storage ingress
// （写入校验）与 egress（行扫描）各一次；materialization 等消费方一律信任
// 已清洗的值，不再逐字段 trim。
func (p ExecutionPlanProposal) Normalized() ExecutionPlanProposal {
	p.ID = strings.TrimSpace(p.ID)
	p.OwnerUserID = strings.TrimSpace(p.OwnerUserID)
	p.SessionKey = strings.TrimSpace(p.SessionKey)
	p.RoomID = strings.TrimSpace(p.RoomID)
	p.ConversationID = strings.TrimSpace(p.ConversationID)
	p.CoordinatorAgentID = strings.TrimSpace(p.CoordinatorAgentID)
	p.RootRoundID = strings.TrimSpace(p.RootRoundID)
	p.RuntimeRoundID = strings.TrimSpace(p.RuntimeRoundID)
	p.AgentRoundID = strings.TrimSpace(p.AgentRoundID)
	p.TargetExecutionID = strings.TrimSpace(p.TargetExecutionID)
	p.BasePlanID = strings.TrimSpace(p.BasePlanID)
	p.GoalID = strings.TrimSpace(p.GoalID)
	p.GoalReservedExecutionID = strings.TrimSpace(p.GoalReservedExecutionID)
	p.ReplacesExecutionID = strings.TrimSpace(p.ReplacesExecutionID)
	p.ReservedExecutionID = strings.TrimSpace(p.ReservedExecutionID)
	p.MaterializationCommandID = strings.TrimSpace(p.MaterializationCommandID)
	p.MaterializedExecutionID = strings.TrimSpace(p.MaterializedExecutionID)
	p.MaterializedPlanID = strings.TrimSpace(p.MaterializedPlanID)
	p.ContentDigest = strings.TrimSpace(p.ContentDigest)
	p.LastError = strings.TrimSpace(p.LastError)
	return p
}

// MarshalExecutionPlanProposalDocument encodes a typed canonical document.
// It deliberately never accepts map[string]any, so digest stability does not depend on map order.
func MarshalExecutionPlanProposalDocument(
	document ExecutionPlanProposalDocument,
) ([]byte, error) {
	return json.Marshal(document)
}

// DigestExecutionPlanProposalDocument returns the document-only v1 SHA-256 identity.
// Persistence and materialization receipts must use DigestExecutionPlanProposalImmutable instead:
// a document digest alone does not bind the proposal to its authority or target fence.
func DigestExecutionPlanProposalDocument(
	document ExecutionPlanProposalDocument,
) (string, error) {
	encoded, err := MarshalExecutionPlanProposalDocument(document)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// ExecutionPlanProposalDigestEnvelope is the canonical immutable authority, round
// provenance and target fence covered by ContentDigest. Mutable lifecycle, receipt and
// timestamp fields deliberately do not participate in the digest.
type ExecutionPlanProposalDigestEnvelope struct {
	Document                ExecutionPlanProposalDocument `json:"document"`
	OwnerUserID             string                        `json:"owner_user_id"`
	SessionKey              string                        `json:"session_key"`
	ScopeKind               ExecutionScopeKind            `json:"scope_kind"`
	RoomID                  string                        `json:"room_id,omitempty"`
	ConversationID          string                        `json:"conversation_id,omitempty"`
	CoordinatorAgentID      string                        `json:"coordinator_agent_id"`
	RootRoundID             string                        `json:"root_round_id,omitempty"`
	RuntimeRoundID          string                        `json:"runtime_round_id,omitempty"`
	AgentRoundID            string                        `json:"agent_round_id,omitempty"`
	TargetExecutionID       string                        `json:"target_execution_id,omitempty"`
	TargetExecutionVersion  int64                         `json:"target_execution_version,omitempty"`
	BasePlanID              string                        `json:"base_plan_id,omitempty"`
	GoalID                  string                        `json:"goal_id,omitempty"`
	GoalObjectiveRevision   int64                         `json:"goal_objective_revision,omitempty"`
	GoalActivationOrigin    GoalActivationOrigin          `json:"goal_activation_origin,omitempty"`
	GoalActivationReason    GoalActivationReason          `json:"goal_activation_reason,omitempty"`
	GoalReservedExecutionID string                        `json:"goal_reserved_execution_id,omitempty"`
	ReplacesExecutionID     string                        `json:"replaces_execution_id,omitempty"`
}

// DigestExecutionPlanProposalImmutable returns the v1 SHA-256 commit identity for a
// proposal's canonical document, permission and round provenance, target/replacement
// fence, and trusted Goal objective/activation fence.
func DigestExecutionPlanProposalImmutable(proposal ExecutionPlanProposal) (string, error) {
	encoded, err := json.Marshal(ExecutionPlanProposalDigestEnvelope{
		Document:                proposal.Document,
		OwnerUserID:             proposal.OwnerUserID,
		SessionKey:              proposal.SessionKey,
		ScopeKind:               proposal.ScopeKind,
		RoomID:                  proposal.RoomID,
		ConversationID:          proposal.ConversationID,
		CoordinatorAgentID:      proposal.CoordinatorAgentID,
		RootRoundID:             proposal.RootRoundID,
		RuntimeRoundID:          proposal.RuntimeRoundID,
		AgentRoundID:            proposal.AgentRoundID,
		TargetExecutionID:       proposal.TargetExecutionID,
		TargetExecutionVersion:  proposal.TargetExecutionVersion,
		BasePlanID:              proposal.BasePlanID,
		GoalID:                  proposal.GoalID,
		GoalObjectiveRevision:   proposal.GoalObjectiveRevision,
		GoalActivationOrigin:    proposal.GoalActivationOrigin,
		GoalActivationReason:    proposal.GoalActivationReason,
		GoalReservedExecutionID: proposal.GoalReservedExecutionID,
		ReplacesExecutionID:     proposal.ReplacesExecutionID,
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
