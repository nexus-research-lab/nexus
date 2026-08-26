// INPUT: current round Actor、host-owned Execution command receipts 与候选 CLI Tool NodeRun。
// OUTPUT: 经 receipt 精确核验并恢复 operation 名、责任分段和审核锚点的 Runtime Graph 节点。
// POS: nexus_runtime transport 与 WorkGraph 结构语义之间的可信桥；不从模型输入授予 authority。
package orchestration

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	runtimeGraphCommandDomainMetadataKey    = "runtime_command_domain"
	runtimeGraphCommandOperationMetadataKey = "runtime_command_operation"
	runtimeGraphCommandRequestIDMetadataKey = "runtime_command_request_id"
	runtimeGraphCommandVerifiedMetadataKey  = "runtime_command_verified"
	runtimeGraphCommandTransportMetadataKey = "runtime_command_transport"
	runtimeGraphCommandActionMetadataKey    = "runtime_command_action"
)

// ObserveRuntimeCommandReceipts reconciles provider Tool nodes only after their
// structured identity matches host-owned receipts. One graph read serves the whole assistant
// checkpoint; providers without Tool lifecycle receive deterministic fallback nodes.
func (s *Service) ObserveRuntimeCommandReceipts(
	ctx context.Context,
	actor ActorContext,
	receipts []runtimecommand.Receipt,
) error {
	if s == nil {
		return nil
	}
	executionReceipts := filterExecutionCommandReceipts(receipts)
	if len(executionReceipts) == 0 {
		return nil
	}
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return nil
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		return err
	}
	defer s.invalidateActor(ctx, actor)
	if err = s.beginRuntimeRound(ctx, actor, false); err != nil {
		return err
	}
	graph, err := repository.GetRuntimeGraph(
		ctx, identity.OwnerUserID, identity.SessionKey, identity.ExecutionID, identity.RootRoundID,
	)
	if err != nil {
		return err
	}
	candidates, verified := indexRuntimeCommandReceiptNodes(graph.Nodes, identity)
	for key := range candidates {
		slices.SortFunc(candidates[key], compareRuntimeCommandCandidate)
	}
	now := s.now().UTC()
	for _, receipt := range executionReceipts {
		key := runtimeCommandReceiptKey(receipt.Domain, receipt.Operation, receipt.RequestID)
		candidateNodes := candidates[key]
		if len(candidateNodes) == 0 && verified[key] {
			continue
		}
		var node protocol.ExecutionRuntimeNodeRun
		matchedCandidate := len(candidateNodes) > 0
		if matchedCandidate {
			node = candidateNodes[0]
			candidates[key] = candidateNodes[1:]
		} else {
			node = runtimeCommandFallbackNode(identity, receipt, now)
		}
		if err = s.applyRuntimeCommandReceipt(
			ctx, repository, actor, identity, receipt, node, matchedCandidate, now,
		); err != nil {
			return err
		}
		verified[key] = true
	}
	return nil
}

func filterExecutionCommandReceipts(receipts []runtimecommand.Receipt) []runtimecommand.Receipt {
	result := make([]runtimecommand.Receipt, 0, len(receipts))
	for _, receipt := range receipts {
		if receipt.Domain == runtimecommand.DomainExecution &&
			strings.TrimSpace(receipt.RequestID) != "" &&
			strings.TrimSpace(receipt.Operation) != "" {
			result = append(result, receipt)
		}
	}
	return result
}

func (s *Service) applyRuntimeCommandReceipt(
	ctx context.Context,
	repository runtimeGraphRepository,
	actor ActorContext,
	identity runtimeGraphIdentity,
	receipt runtimecommand.Receipt,
	node protocol.ExecutionRuntimeNodeRun,
	matchedCandidate bool,
	now time.Time,
) error {
	if node.Metadata == nil {
		node.Metadata = make(map[string]any)
	}
	node.Name = receipt.Operation
	node.Description = "Nexus execution command"
	node.ExecutionID = firstNonEmpty(receipt.ExecutionID, actor.ExecutionID, node.ExecutionID)
	node.ResultSummary = strings.TrimSpace(receipt.Message)
	node.ErrorCode = strings.TrimSpace(receipt.ReasonCode)
	node.Metadata[runtimeGraphCommandDomainMetadataKey] = receipt.Domain
	node.Metadata[runtimeGraphCommandOperationMetadataKey] = receipt.Operation
	node.Metadata[runtimeGraphCommandRequestIDMetadataKey] = receipt.RequestID
	node.Metadata[runtimeGraphCommandVerifiedMetadataKey] = true
	node.Metadata[runtimeGraphCommandTransportMetadataKey] = true
	node.Metadata[runtimeGraphCommandActionMetadataKey] = runtimecommand.ActionInvoke
	if receipt.Outcome != "" {
		node.Metadata["mutation_outcome"] = receipt.Outcome
	}
	if receipt.Outcome == string(protocol.MutationResultRejected) {
		node.Status = protocol.ExecutionRuntimeNodeFailed
		node.Failed = true
		node.ErrorSummary = strings.TrimSpace(receipt.Message)
	} else if node.Status == "" || node.Status == protocol.ExecutionRuntimeNodeRunning {
		node.Status = protocol.ExecutionRuntimeNodeSucceeded
	}
	if node.FinishedAt == nil {
		node.FinishedAt = &now
	}
	node.UpdatedAt = now

	segment := s.runtimeCommandReceiptSegment(ctx, actor, identity, receipt)
	if segment.valid() {
		if err := repository.BindRuntimeGraphRoundExecution(
			ctx, identity.OwnerUserID, identity.SessionKey, identity.AgentRoundID, segment.ExecutionID,
		); err != nil {
			return err
		}
		applyRuntimeExecutionSegment(node.Metadata, segment)
		node.Metadata[runtimeGraphSegmentBoundaryKey] = runtimeGraphSegmentBoundaryAssign
	} else if actor.ScopeKind == protocol.ExecutionScopeDM && receipt.Operation == "assign_work" {
		clearRuntimeExecutionSegment(node.Metadata)
		node.Metadata[runtimeGraphSegmentBoundaryKey] = runtimeGraphSegmentBoundaryUnresolved
	}
	if err := repository.UpsertRuntimeGraphNode(ctx, node); err != nil {
		return err
	}
	if matchedCandidate {
		return nil
	}
	edge := protocol.ExecutionRuntimeEdgeRun{
		GraphID: identity.GraphID, OwnerUserID: identity.OwnerUserID, SessionKey: identity.SessionKey,
		SourceNodeID: runtimeGraphNodeID(identity, protocol.ExecutionRuntimeNodeAgent, identity.AgentRoundID),
		TargetNodeID: node.ID, Kind: protocol.ExecutionRuntimeEdgeInvoke, CreatedAt: now,
	}
	edge.ID = runtimeGraphEdgeID(edge)
	return repository.UpsertRuntimeGraphEdge(ctx, edge)
}

func indexRuntimeCommandReceiptNodes(
	nodes []protocol.ExecutionRuntimeNodeRun,
	identity runtimeGraphIdentity,
) (map[string][]protocol.ExecutionRuntimeNodeRun, map[string]bool) {
	candidates := make(map[string][]protocol.ExecutionRuntimeNodeRun)
	verified := make(map[string]bool)
	for _, node := range nodes {
		if node.Kind != protocol.ExecutionRuntimeNodeTool || node.AgentRoundID != identity.AgentRoundID {
			continue
		}
		domain := runtimeGraphMetadataString(node, runtimeGraphCommandDomainMetadataKey)
		operation := runtimeGraphMetadataString(node, runtimeGraphCommandOperationMetadataKey)
		requestID := runtimeGraphMetadataString(node, runtimeGraphCommandRequestIDMetadataKey)
		if domain == "" || operation == "" || requestID == "" {
			continue
		}
		key := runtimeCommandReceiptKey(domain, operation, requestID)
		if runtimeGraphMetadataBool(node, runtimeGraphCommandVerifiedMetadataKey) {
			verified[key] = true
		} else {
			candidates[key] = append(candidates[key], node)
		}
	}
	return candidates, verified
}

func compareRuntimeCommandCandidate(left, right protocol.ExecutionRuntimeNodeRun) int {
	if order := left.UpdatedAt.Compare(right.UpdatedAt); order != 0 {
		return order
	}
	return strings.Compare(left.ID, right.ID)
}

func runtimeCommandReceiptKey(domain, operation, requestID string) string {
	return strings.Join([]string{
		strings.TrimSpace(domain), strings.TrimSpace(operation), strings.TrimSpace(requestID),
	}, "\x00")
}

func runtimeCommandFallbackNode(
	identity runtimeGraphIdentity,
	receipt runtimecommand.Receipt,
	now time.Time,
) protocol.ExecutionRuntimeNodeRun {
	subjectID := "runtime-command:" + receipt.RequestID
	return protocol.ExecutionRuntimeNodeRun{
		ID:      runtimeGraphNodeID(identity, protocol.ExecutionRuntimeNodeTool, subjectID),
		GraphID: identity.GraphID, OwnerUserID: identity.OwnerUserID, SessionKey: identity.SessionKey,
		ExecutionID: receipt.ExecutionID, Kind: protocol.ExecutionRuntimeNodeTool,
		SubjectID: subjectID, ParentSubjectID: identity.AgentRoundID,
		RootRoundID: identity.RootRoundID, RuntimeRoundID: identity.RuntimeRoundID,
		AgentRoundID: identity.AgentRoundID, AgentID: identity.AgentID,
		Status: protocol.ExecutionRuntimeNodeSucceeded, StartedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runtimeGraphCommandDomainMetadataKey:    receipt.Domain,
			runtimeGraphCommandOperationMetadataKey: receipt.Operation,
			runtimeGraphCommandRequestIDMetadataKey: receipt.RequestID,
			runtimeGraphCommandTransportMetadataKey: true,
			runtimeGraphCommandActionMetadataKey:    runtimecommand.ActionInvoke,
		},
	}
}

func (s *Service) runtimeCommandReceiptSegment(
	ctx context.Context,
	actor ActorContext,
	identity runtimeGraphIdentity,
	receipt runtimecommand.Receipt,
) runtimeExecutionSegment {
	if !runtimeGraphAssignmentBoundaryOperation(receipt.Operation) || !receipt.Applied() {
		return runtimeExecutionSegment{}
	}
	source := runtimeGraphAssignmentBoundarySource(receipt.Operation)
	segment := runtimeExecutionSegment{
		ExecutionID: receipt.ExecutionID, WorkItemID: receipt.WorkItemID,
		AssignmentID: receipt.AssignmentID, AttemptID: receipt.AttemptID,
		Source: source,
	}
	if segment.valid() {
		return segment
	}
	if actor.ScopeKind != protocol.ExecutionScopeDM {
		return runtimeExecutionSegment{}
	}
	return s.runtimeExecutionSegmentFromChangedRefs(
		ctx,
		actor,
		identity,
		receipt.ExecutionID,
		receipt.Changed,
		source,
	)
}

func runtimeGraphMetadataBool(item protocol.ExecutionRuntimeNodeRun, key string) bool {
	value, _ := item.Metadata[key].(bool)
	return value
}
