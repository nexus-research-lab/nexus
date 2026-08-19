// INPUT: 单个 strict Nexus Plan Document、显式 Goal binding intent、trusted actor/scope/Goal authority 与当前 Execution snapshot。
// OUTPUT: 按 intent/document/current Execution 选择并 canonicalize root boundary、跨 round 可恢复且绑定 exact target fence 的 sealed ExecutionPlanProposal。
// POS: Provider 字符串传输与权威 Plan materialization 之间的非权威应用服务边界；ambient Goal 不参与 proposal sealing。
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// PlanProposalRepository 是 sealed proposal saga 使用的独立持久化端口。
// 它不加入主 Repository，避免不涉及 proposal 的测试 fake 被迫实现无关能力。
type PlanProposalRepository interface {
	CreateOrGetPlanProposal(
		context.Context,
		orchestrationstore.CreateOrGetPlanProposalCommand,
	) (*protocol.ExecutionPlanProposal, error)
	GetPlanProposal(
		context.Context,
		orchestrationstore.GetPlanProposalQuery,
	) (*protocol.ExecutionPlanProposal, error)
	MarkPlanProposalMaterializing(
		context.Context,
		orchestrationstore.MarkPlanProposalMaterializingCommand,
	) (*protocol.ExecutionPlanProposal, error)
	ClaimPlanProposalMaterializing(
		context.Context,
		orchestrationstore.ClaimPlanProposalMaterializingCommand,
	) (*protocol.ExecutionPlanProposal, error)
	MarkPlanProposalMaterialized(
		context.Context,
		orchestrationstore.MarkPlanProposalMaterializedCommand,
	) (*protocol.ExecutionPlanProposal, error)
	MarkPlanProposalConfirmation(
		context.Context,
		orchestrationstore.MarkPlanProposalConfirmationCommand,
	) (*protocol.ExecutionPlanProposal, error)
	MarkPlanProposalBlocked(
		context.Context,
		orchestrationstore.MarkPlanProposalBlockedCommand,
	) (*protocol.ExecutionPlanProposal, error)
	SchedulePlanProposalRetry(
		context.Context,
		orchestrationstore.SchedulePlanProposalRetryCommand,
	) (*protocol.ExecutionPlanProposal, error)
	ListRecoverablePlanProposals(
		context.Context,
		orchestrationstore.ListRecoverablePlanProposalsQuery,
	) ([]protocol.ExecutionPlanProposal, error)
}

// PlanMaterializationReceiptReader proves that the exact stable proposal
// command activated a Plan. Semantic graph equality is insufficient because
// two proposals may intentionally describe the same graph under different fences.
type PlanMaterializationReceiptReader interface {
	FindPlanMaterializationReceipt(
		context.Context,
		string,
		string,
	) (string, error)
}

// PlanGoalBindingIntent 控制 proposal 是否使用当前轮次的 exact Goal authority。
// 模型不能提供 Goal identity；replan/replace 只能继承当前 Execution boundary。
type PlanGoalBindingIntent string

const (
	PlanGoalBindingNone    PlanGoalBindingIntent = "none"
	PlanGoalBindingCurrent PlanGoalBindingIntent = "current"
	PlanGoalBindingInherit PlanGoalBindingIntent = "inherit"
)

// PreparePlanExecutionInput 接收 provider 能稳定传递的完整文本和一个有界标量 intent。
// Target、scope、Goal identity 和 coordinator identity 全部从 trusted runtime 派生。
type PreparePlanExecutionInput struct {
	CommandID    string
	PlanDocument string
	GoalBinding  PlanGoalBindingIntent
}

// PreparePlanExecution 解析并 seal 一份完整 proposal；它可以在 Plan Mode 中
// 写入非权威 proposal，但绝不创建或修改 Execution、Plan、Goal。
func (s *Service) PreparePlanExecution(
	ctx context.Context,
	actor ActorContext,
	input PreparePlanExecutionInput,
) (*protocol.ExecutionPlanProposal, error) {
	defer s.WakeOrchestrationRecovery()
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if err := requireExecutionCoordinator(actor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return nil, domainError(ErrorCodeInvalidInput, "command_id is required")
	}
	if strings.TrimSpace(actor.RootRoundID) == "" {
		return nil, domainError(ErrorCodeInvalidInput, "root_round_id is required to seal a recoverable Plan proposal")
	}
	if s == nil || s.planProposals == nil {
		return nil, errors.New("execution plan proposal repository is unavailable")
	}
	document, draft, err := ParseExecutionPlanDocument(input.PlanDocument)
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			return nil, err
		}
		return nil, domainError(ErrorCodePlanDocumentInvalid, err.Error())
	}
	goalBinding, err := normalizePlanGoalBindingIntent(document.Operation, input.GoalBinding, actor)
	if err != nil {
		return nil, err
	}

	lookupActor := actor
	lookupActor.ExecutionID = ""
	snapshot, err := s.GetCurrent(ctx, lookupActor)
	if err != nil {
		return nil, err
	}
	var activation *ExplicitGoalActivation
	if document.Operation == protocol.ExecutionPlanProposalCreate &&
		snapshot == nil && goalBinding == PlanGoalBindingCurrent {
		activation, err = s.resolveProposalGoalActivation(ctx, actor)
		if err != nil {
			return nil, err
		}
		if activation != nil {
			// The active Goal owns the persistent objective. The provider may omit
			// or paraphrase the transport field, but the sealed document and digest
			// must carry the exact server-owned Goal boundary.
			document.Objective = strings.TrimSpace(activation.Objective)
		}
	}
	if err = s.validatePreparedPlanProposal(actor, document, draft, snapshot); err != nil {
		return nil, err
	}
	if document.Operation == protocol.ExecutionPlanProposalReplan && snapshot != nil {
		// A replan input may omit immutable Execution boundary fields for ergonomics,
		// but the sealed canonical document must be self-contained and digest the
		// exact boundary it preserves.
		document.Objective = strings.TrimSpace(snapshot.Execution.Objective)
		document.CompletionCriteria = slices.Clone(snapshot.Execution.CompletionCriteria)
	}

	now := s.currentTime()
	proposal := protocol.ExecutionPlanProposal{
		OwnerUserID:        strings.TrimSpace(actor.OwnerUserID),
		SessionKey:         strings.TrimSpace(actor.SessionKey),
		ScopeKind:          normalizedProposalScope(actor.ScopeKind),
		RoomID:             strings.TrimSpace(actor.RoomID),
		ConversationID:     strings.TrimSpace(actor.ConversationID),
		CoordinatorAgentID: strings.TrimSpace(actor.AgentID),
		RootRoundID:        strings.TrimSpace(actor.RootRoundID),
		RuntimeRoundID:     strings.TrimSpace(actor.RuntimeRoundID),
		AgentRoundID:       strings.TrimSpace(actor.AgentRoundID),
		Document:           document,
		Status:             protocol.ExecutionPlanProposalStatusSealed,
		Version:            1,
		ConfirmationState:  protocol.ExecutionPlanProposalConfirmationNone,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if snapshot != nil {
		proposal.TargetExecutionID = strings.TrimSpace(snapshot.Execution.ID)
		proposal.TargetExecutionVersion = snapshot.Execution.Version
		if snapshot.Plan != nil {
			proposal.BasePlanID = strings.TrimSpace(snapshot.Plan.ID)
		}
		proposal.GoalID = strings.TrimSpace(snapshot.Execution.GoalID)
		proposal.GoalObjectiveRevision = snapshot.Execution.GoalObjectiveRevision
		proposal.GoalActivationOrigin = snapshot.Execution.GoalActivationOrigin
		proposal.GoalActivationReason = snapshot.Execution.GoalActivationReason
	} else {
		if activation != nil {
			proposal.GoalID = activation.GoalID
			proposal.GoalObjectiveRevision = activation.GoalObjectiveRevision
			proposal.GoalActivationOrigin = activation.ActivationOrigin
			proposal.GoalActivationReason = activation.ActivationReason
			proposal.GoalReservedExecutionID = strings.TrimSpace(activation.ReservedExecutionID)
			proposal.ReplacesExecutionID = strings.TrimSpace(activation.ReplacesExecutionID)
		}
	}
	if document.Operation == protocol.ExecutionPlanProposalReplace {
		proposal.ReplacesExecutionID = proposal.TargetExecutionID
	}
	if err = validateProposalGoalFence(proposal.GoalID, proposal.GoalObjectiveRevision); err != nil {
		return nil, err
	}

	digest, err := protocol.DigestExecutionPlanProposalImmutable(proposal)
	if err != nil {
		return nil, fmt.Errorf("digest execution plan proposal: %w", err)
	}
	proposal.ContentDigest = digest
	proposal.ID = deterministicPlanProposalID(input.CommandID, digest)
	created, err := s.planProposals.CreateOrGetPlanProposal(
		ctx,
		orchestrationstore.CreateOrGetPlanProposalCommand{Proposal: proposal},
	)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) resolveProposalGoalActivation(
	ctx context.Context,
	actor ActorContext,
) (*ExplicitGoalActivation, error) {
	if !actorHasExactGoalAuthority(actor) {
		return nil, nil
	}
	resolver, ok := s.explicitGoalGateway.(ExplicitGoalActivationResolver)
	if !ok {
		return nil, domainError(ErrorCodeGoalBindingConflict,
			"trusted Goal activation provenance is unavailable for Plan preparation")
	}
	activation, err := resolver.ResolveExplicitGoalActivation(ctx, ExplicitGoalActivationRequest{
		ExistingGoalID:        strings.TrimSpace(actor.GoalID),
		GoalObjectiveRevision: actor.GoalObjectiveRevision,
		OwnerUserID:           strings.TrimSpace(actor.OwnerUserID),
		SessionKey:            strings.TrimSpace(actor.SessionKey),
		ScopeKind:             normalizedProposalScope(actor.ScopeKind),
		ConversationID:        strings.TrimSpace(actor.ConversationID),
		AgentID:               strings.TrimSpace(actor.AgentID),
	})
	if err != nil {
		return nil, mapExplicitGoalGatewayError(err)
	}
	if activation == nil {
		return nil, domainError(
			ErrorCodeGoalBindingConflict,
			"exact Goal authority disappeared during Plan preparation",
		)
	}
	activation.GoalID = strings.TrimSpace(activation.GoalID)
	activation.ReservedExecutionID = strings.TrimSpace(activation.ReservedExecutionID)
	activation.ReplacesExecutionID = strings.TrimSpace(activation.ReplacesExecutionID)
	activation.Objective = strings.TrimSpace(activation.Objective)
	if activation.GoalID == "" || activation.GoalObjectiveRevision <= 0 ||
		activation.Objective == "" ||
		activation.ActivationOrigin == "" || activation.ActivationReason == "" ||
		(strings.TrimSpace(actor.GoalID) != "" && activation.GoalID != strings.TrimSpace(actor.GoalID)) ||
		(actor.GoalObjectiveRevision > 0 &&
			activation.GoalObjectiveRevision != actor.GoalObjectiveRevision) ||
		(activation.ReplacesExecutionID != "" && activation.ReservedExecutionID == "") ||
		(activation.ReservedExecutionID != "" &&
			activation.ReplacesExecutionID == activation.ReservedExecutionID) {
		return nil, domainError(
			ErrorCodeGoalBindingConflict,
			"exact Goal authority changed or returned an incomplete Execution fence during Plan preparation",
		)
	}
	return activation, nil
}

func normalizePlanGoalBindingIntent(
	operation protocol.ExecutionPlanProposalOperation,
	requested PlanGoalBindingIntent,
	actor ActorContext,
) (PlanGoalBindingIntent, error) {
	requested = PlanGoalBindingIntent(strings.TrimSpace(string(requested)))
	switch operation {
	case protocol.ExecutionPlanProposalCreate:
		if requested == "" {
			if actorHasExactGoalAuthority(actor) {
				return PlanGoalBindingCurrent, nil
			}
			return PlanGoalBindingNone, nil
		}
		switch requested {
		case PlanGoalBindingNone:
			return requested, nil
		case PlanGoalBindingCurrent:
			if !actorHasExactGoalAuthority(actor) {
				return "", domainError(
					ErrorCodeGoalBindingConflict,
					"goal_binding current requires exact Goal id and objective revision authority in this round",
				)
			}
			return requested, nil
		case PlanGoalBindingInherit:
			return "", domainError(
				ErrorCodeInvalidInput,
				"goal_binding inherit is unavailable for operation: create",
			)
		default:
			return "", invalidPlanGoalBindingIntentError()
		}

	case protocol.ExecutionPlanProposalReplan, protocol.ExecutionPlanProposalReplace:
		if requested == "" || requested == PlanGoalBindingInherit {
			return PlanGoalBindingInherit, nil
		}
		if requested == PlanGoalBindingNone || requested == PlanGoalBindingCurrent {
			return "", domainError(
				ErrorCodeInvalidInput,
				"operation: replan and operation: replace require goal_binding inherit",
			)
		}
		return "", invalidPlanGoalBindingIntentError()

	default:
		return "", domainError(ErrorCodePlanDocumentInvalid, "unsupported plan operation")
	}
}

func actorHasExactGoalAuthority(actor ActorContext) bool {
	return strings.TrimSpace(actor.GoalID) != "" && actor.GoalObjectiveRevision > 0
}

func invalidPlanGoalBindingIntentError() error {
	return domainError(
		ErrorCodeInvalidInput,
		"goal_binding must be none, current, or inherit",
	)
}

func (s *Service) validatePreparedPlanProposal(
	actor ActorContext,
	document protocol.ExecutionPlanProposalDocument,
	draft PlanDraft,
	snapshot *protocol.ExecutionSnapshot,
) error {
	switch document.Operation {
	case protocol.ExecutionPlanProposalCreate:
		if snapshot != nil {
			return domainError(
				ErrorCodePlanProposalStale,
				"a current Execution already exists; prepare a replan or explicit replacement instead",
			)
		}
		if document.SupersedeActiveWork || strings.TrimSpace(document.ReplacementReason) != "" {
			return domainError(ErrorCodeInvalidInput, "create cannot carry replan or replacement controls")
		}
		if _, _, err := validateExecutionBoundary(document.Objective, document.CompletionCriteria); err != nil {
			return err
		}
		for _, item := range draft.Items {
			if strings.TrimSpace(item.ExistingWorkItemID) != "" {
				return newDomainError(
					ErrorCodeInvalidInput,
					"create cannot carry an existing Work Item identity",
					item.LogicalKey,
					item.ExistingWorkItemID,
				)
			}
		}
		return nil

	case protocol.ExecutionPlanProposalReplan:
		if snapshot == nil {
			return domainError(ErrorCodeNoCurrentExecution,
				"replan requires a current Execution; when inspect returns none, seal the successor or fresh WorkGraph with operation: create")
		}
		if err := requireCoordinator(actor, snapshot); err != nil {
			return err
		}
		if !isCurrentExecutionStatus(snapshot.Execution.Status) {
			return terminalExecutionError()
		}
		if strings.TrimSpace(document.ReplacementReason) != "" {
			return domainError(ErrorCodeInvalidInput, "replan cannot carry replacement_reason")
		}
		if strings.TrimSpace(document.RevisionReason) == "" {
			return domainError(ErrorCodeInvalidInput, "replan requires a non-empty revision_reason")
		}
		if err := validateOrdinaryReplanBoundary(
			snapshot,
			document.Objective,
			document.CompletionCriteria,
		); err != nil {
			return err
		}
		return validatePreparedReplan(snapshot, document, draft)

	case protocol.ExecutionPlanProposalReplace:
		if snapshot == nil {
			return domainError(ErrorCodeNoCurrentExecution,
				"replace requires a current transient Execution; when inspect returns none, seal the successor or fresh WorkGraph with operation: create")
		}
		if err := requireCoordinator(actor, snapshot); err != nil {
			return err
		}
		_, _, err := validateReplacementBoundary(snapshot, PlanExecutionInput{
			ExecutionID:             snapshot.Execution.ID,
			SnapshotRevision:        snapshot.Execution.Version,
			Objective:               document.Objective,
			CompletionCriteria:      document.CompletionCriteria,
			ReplaceCurrentExecution: true,
			ReplacementReason:       document.ReplacementReason,
			SupersedeActiveWork:     document.SupersedeActiveWork,
			Draft:                   draft,
		})
		return err

	default:
		return domainError(ErrorCodePlanDocumentInvalid, "unsupported plan operation")
	}
}

func validatePreparedReplan(
	snapshot *protocol.ExecutionSnapshot,
	document protocol.ExecutionPlanProposalDocument,
	draft PlanDraft,
) error {
	if document.SupersedeActiveWork && strings.TrimSpace(document.RevisionReason) == "" {
		return domainError(
			ErrorCodeInvalidInput,
			"supersede_active_work requires a non-empty revision_reason",
		)
	}
	if hasUnreviewedSubmission(snapshot) {
		return domainError(
			ErrorCodeCompletionBlocked,
			"review pending submissions before replacing the active Plan",
		)
	}
	monotonic, err := planDraftMonotonicallyExtendsSnapshot(snapshot, draft)
	if err != nil {
		return err
	}
	if !monotonic && !document.SupersedeActiveWork {
		return domainError(
			ErrorCodeCompletionBlocked,
			"changing existing Plan nodes or dependencies requires supersede_active_work and revision_reason",
		)
	}
	if document.SupersedeActiveWork {
		return nil
	}
	for _, assignment := range snapshot.Assignments {
		if currentAssignment(assignment) {
			return domainError(
				ErrorCodeCompletionBlocked,
				"finish or review current assignments before replacing the active Plan",
			)
		}
	}
	return nil
}

func normalizedProposalScope(scope protocol.ExecutionScopeKind) protocol.ExecutionScopeKind {
	if scope == "" {
		return protocol.ExecutionScopeDM
	}
	return scope
}

func validateProposalGoalFence(goalID string, revision int64) error {
	goalID = strings.TrimSpace(goalID)
	if (goalID == "") != (revision <= 0) {
		return domainError(
			ErrorCodeGoalBindingConflict,
			"trusted Goal identity and objective revision must be present together",
		)
	}
	return nil
}

func deterministicPlanProposalID(commandID, digest string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(commandID) + "\x00" + strings.TrimSpace(digest)))
	return "plan_proposal_" + hex.EncodeToString(hash[:20])
}

func deterministicProposalExecutionID(proposalID, digest string) string {
	hash := sha256.Sum256([]byte("execution\x00" + proposalID + "\x00" + digest))
	return "execution_" + hex.EncodeToString(hash[:20])
}

func deterministicProposalCommandID(proposalID, digest string) string {
	hash := sha256.Sum256([]byte("materialize\x00" + proposalID + "\x00" + digest))
	return "plan_proposal_" + hex.EncodeToString(hash[:])
}

func proposalAccess(
	actor ActorContext,
	proposalID string,
) orchestrationstore.PlanProposalAccess {
	return orchestrationstore.PlanProposalAccess{
		ProposalID:         strings.TrimSpace(proposalID),
		OwnerUserID:        strings.TrimSpace(actor.OwnerUserID),
		SessionKey:         strings.TrimSpace(actor.SessionKey),
		ScopeKind:          normalizedProposalScope(actor.ScopeKind),
		RoomID:             strings.TrimSpace(actor.RoomID),
		ConversationID:     strings.TrimSpace(actor.ConversationID),
		CoordinatorAgentID: strings.TrimSpace(actor.AgentID),
	}
}

func canonicalDraftFromProposal(
	document protocol.ExecutionPlanProposalDocument,
) PlanDraft {
	draft := PlanDraft{
		RevisionReason: strings.TrimSpace(document.RevisionReason),
		Items:          make([]PlanWorkItemDraft, len(document.Items)),
	}
	for index, item := range document.Items {
		dependencies := make([]PlanDependencyDraft, len(item.DependsOn))
		for dependencyIndex, dependency := range item.DependsOn {
			dependencies[dependencyIndex] = PlanDependencyDraft{
				LogicalKey: dependency.LogicalKey,
				Kind:       dependency.Kind,
			}
		}
		draft.Items[index] = PlanWorkItemDraft{
			LogicalKey:         item.LogicalKey,
			ExistingWorkItemID: item.ExistingWorkItemID,
			Kind:               item.Kind,
			Subject:            item.Subject,
			Objective:          item.Objective,
			Deliverable:        item.Deliverable,
			AcceptanceCriteria: slices.Clone(item.AcceptanceCriteria),
			Required:           item.Required,
			Terminal:           item.Terminal,
			ParentLogicalKey:   item.ParentLogicalKey,
			DependsOn:          dependencies,
			InputRefs:          slices.Clone(item.InputRefs),
			OutputScopes:       slices.Clone(item.OutputScopes),
		}
	}
	return draft
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
