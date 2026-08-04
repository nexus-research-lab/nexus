// INPUT: Nexus actor/round identity 与 Bridge 自动派生的 runtime lifecycle 事件。
// OUTPUT: fail-open 的 Agent/Tool/Subagent NodeRun、运行边与 WorkGraph 合并投影。
// POS: 模型不可见的运行观测层；不要求模型调用 MCP 汇报工具或子智能体状态。
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type runtimeGraphRepository interface {
	UpsertRuntimeGraphNode(context.Context, protocol.ExecutionRuntimeNodeRun) error
	UpsertRuntimeGraphEdge(context.Context, protocol.ExecutionRuntimeEdgeRun) error
	ReconcileRuntimeGraphAgent(context.Context, string, string, string, string, time.Time) error
	FinishRuntimeGraphRound(context.Context, string, string, string, protocol.ExecutionRuntimeNodeStatus, time.Time) error
	GetRuntimeGraph(context.Context, string, string, string, string) (protocol.ExecutionRuntimeGraph, error)
}

// BeginRuntimeRound 建立 planless 与 managed Execution 共用的 root AgentRun。
func (s *Service) BeginRuntimeRound(ctx context.Context, actor ActorContext) error {
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return nil
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	descriptor := runtimeAgentNodeDescriptor(actor)
	if err = repository.ReconcileRuntimeGraphAgent(
		ctx,
		identity.OwnerUserID,
		identity.SessionKey,
		identity.AgentID,
		identity.AgentRoundID,
		now,
	); err != nil {
		return err
	}
	return repository.UpsertRuntimeGraphNode(ctx, protocol.ExecutionRuntimeNodeRun{
		ID:             runtimeGraphNodeID(identity, protocol.ExecutionRuntimeNodeAgent, identity.AgentRoundID),
		GraphID:        identity.GraphID,
		OwnerUserID:    identity.OwnerUserID,
		SessionKey:     identity.SessionKey,
		ExecutionID:    identity.ExecutionID,
		Kind:           protocol.ExecutionRuntimeNodeAgent,
		SubjectID:      identity.AgentRoundID,
		RootRoundID:    identity.RootRoundID,
		RuntimeRoundID: identity.RuntimeRoundID,
		AgentRoundID:   identity.AgentRoundID,
		AgentID:        identity.AgentID,
		Name:           descriptor.Name,
		Description:    descriptor.Description,
		Status:         protocol.ExecutionRuntimeNodeRunning,
		StartedAt:      now,
		UpdatedAt:      now,
		Metadata:       descriptor.Metadata,
	})
}

// ObserveRuntimeMessage 持久化 Bridge 的确定性 lifecycle；写失败由调用方记录，
// 不得把已经执行过的真实工具判定为失败并重放。
func (s *Service) ObserveRuntimeMessage(
	ctx context.Context,
	actor ActorContext,
	message sdkprotocol.ReceivedMessage,
) error {
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return nil
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		return err
	}
	events := message.RuntimeLifecycle
	if len(events) == 0 {
		events = sdkprotocol.DeriveRuntimeLifecycleEvents(message)
	}
	if len(events) == 0 {
		return nil
	}
	if err = s.BeginRuntimeRound(ctx, actor); err != nil {
		return err
	}
	now := s.now().UTC()
	rootNodeID := runtimeGraphNodeID(
		identity,
		protocol.ExecutionRuntimeNodeAgent,
		identity.AgentRoundID,
	)
	parentNodeBySubject := map[string]string{
		identity.AgentRoundID: rootNodeID,
	}
	if graph, graphErr := repository.GetRuntimeGraph(
		ctx,
		identity.OwnerUserID,
		identity.SessionKey,
		identity.ExecutionID,
		identity.RootRoundID,
	); graphErr == nil {
		indexRuntimeGraphParents(parentNodeBySubject, graph.Nodes)
	}
	for _, event := range events {
		if err = s.observeRuntimeSubagentLifecycle(ctx, actor, event); err != nil {
			return err
		}
		nodeKind, edgeKind, mapErr := runtimeGraphKinds(event.NodeKind)
		if mapErr != nil {
			continue
		}
		status := runtimeGraphStatus(event)
		nodeID := runtimeGraphNodeID(identity, nodeKind, event.SubjectID)
		sourceNodeID := rootNodeID
		if parentNodeID := parentNodeBySubject[strings.TrimSpace(event.ParentSubjectID)]; parentNodeID != "" {
			sourceNodeID = parentNodeID
		}
		finishedAt := (*time.Time)(nil)
		if event.Phase == sdkprotocol.RuntimeLifecycleFinished {
			finishedAt = &now
		}
		metadata := make(map[string]any, len(event.Metadata)+1)
		for key, value := range event.Metadata {
			metadata[key] = value
		}
		metadata["bridge_event_id"] = event.EventID
		if err = repository.UpsertRuntimeGraphNode(ctx, protocol.ExecutionRuntimeNodeRun{
			ID:              nodeID,
			GraphID:         identity.GraphID,
			OwnerUserID:     identity.OwnerUserID,
			SessionKey:      identity.SessionKey,
			ExecutionID:     identity.ExecutionID,
			Kind:            nodeKind,
			SubjectID:       strings.TrimSpace(event.SubjectID),
			ParentSubjectID: strings.TrimSpace(event.ParentSubjectID),
			RootRoundID:     identity.RootRoundID,
			RuntimeRoundID:  identity.RuntimeRoundID,
			AgentRoundID:    identity.AgentRoundID,
			AgentID:         firstNonEmpty(event.AgentID, identity.AgentID),
			Name:            event.Name,
			Description:     event.Description,
			Status:          status,
			Failed:          event.Failed,
			StartedAt:       now,
			UpdatedAt:       now,
			FinishedAt:      finishedAt,
			Metadata:        metadata,
		}); err != nil {
			return err
		}
		edge := protocol.ExecutionRuntimeEdgeRun{
			GraphID:      identity.GraphID,
			OwnerUserID:  identity.OwnerUserID,
			SessionKey:   identity.SessionKey,
			SourceNodeID: sourceNodeID,
			TargetNodeID: nodeID,
			Kind:         edgeKind,
			CreatedAt:    now,
		}
		edge.ID = runtimeGraphEdgeID(edge)
		if err = repository.UpsertRuntimeGraphEdge(ctx, edge); err != nil {
			return err
		}
		subjectID := strings.TrimSpace(event.SubjectID)
		if _, alreadyBound := parentNodeBySubject[subjectID]; !alreadyBound || nodeKind == protocol.ExecutionRuntimeNodeSubagent {
			parentNodeBySubject[subjectID] = nodeID
		}
		if nodeKind == protocol.ExecutionRuntimeNodeSubagent {
			if toolUseID := strings.TrimSpace(event.Metadata["tool_use_id"]); toolUseID != "" {
				parentNodeBySubject[toolUseID] = nodeID
			}
		}
	}
	return nil
}

func indexRuntimeGraphParents(
	result map[string]string,
	nodes []protocol.ExecutionRuntimeNodeRun,
) {
	for _, node := range nodes {
		if subjectID := strings.TrimSpace(node.SubjectID); subjectID != "" {
			result[subjectID] = node.ID
		}
	}
	// A child runtime commonly reports the parent Agent tool-use identity.
	// Prefer the resulting Subagent node over the launch Tool node so child
	// tools render inside the actual child context.
	for _, node := range nodes {
		if node.Kind != protocol.ExecutionRuntimeNodeSubagent {
			continue
		}
		toolUseID, _ := node.Metadata["tool_use_id"].(string)
		if toolUseID = strings.TrimSpace(toolUseID); toolUseID != "" {
			result[toolUseID] = node.ID
		}
	}
}

func (s *Service) observeRuntimeSubagentLifecycle(
	ctx context.Context,
	actor ActorContext,
	event sdkprotocol.RuntimeLifecycleEvent,
) error {
	if event.NodeKind != sdkprotocol.RuntimeLifecycleNodeSubagent {
		return nil
	}
	toolUseID := strings.TrimSpace(event.Metadata["tool_use_id"])
	if toolUseID == "" {
		return nil
	}
	input := SubagentLifecycleInput{
		ToolUseID:  toolUseID,
		SDKAgentID: strings.TrimSpace(event.AgentID),
		AgentType:  strings.TrimSpace(event.Name),
	}
	var (
		result SubagentAdmissionResult
		err    error
	)
	switch event.Phase {
	case sdkprotocol.RuntimeLifecycleStarted:
		result, err = s.ObserveSubagentStart(ctx, actor, input)
	case sdkprotocol.RuntimeLifecycleFinished:
		status := strings.ToLower(strings.TrimSpace(event.Status))
		input.Interrupted = status == "cancelled" || status == "canceled" ||
			status == "interrupted" || status == "stopped" || status == "aborted"
		if event.Failed {
			input.Error = firstNonEmpty(strings.TrimSpace(event.Description), status, "subagent failed")
		}
		result, err = s.ObserveSubagentStop(ctx, actor, input)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	// Lifecycle observation is fail-open for the runtime graph. A missing or
	// stale managed binding must not cause the already-produced SDK message to
	// be replayed; durable reconciliation can still close the old Attempt.
	_ = result
	return nil
}

// FinishRuntimeRound 单调收口 root AgentRun；Tool/Subagent 各自终态仍来自 Bridge。
func (s *Service) FinishRuntimeRound(
	ctx context.Context,
	actor ActorContext,
	terminalStatus string,
	failureReason string,
) error {
	repository, ok := s.repository.(runtimeGraphRepository)
	if !ok || repository == nil {
		return nil
	}
	identity, err := runtimeGraphIdentityFromActor(actor)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	status := normalizeRuntimeGraphTerminalStatus(terminalStatus, failureReason)
	descriptor := runtimeAgentNodeDescriptor(actor)
	description := descriptor.Description
	if reason := strings.TrimSpace(failureReason); reason != "" {
		description = reason
	}
	if err = repository.UpsertRuntimeGraphNode(ctx, protocol.ExecutionRuntimeNodeRun{
		ID:             runtimeGraphNodeID(identity, protocol.ExecutionRuntimeNodeAgent, identity.AgentRoundID),
		GraphID:        identity.GraphID,
		OwnerUserID:    identity.OwnerUserID,
		SessionKey:     identity.SessionKey,
		ExecutionID:    identity.ExecutionID,
		Kind:           protocol.ExecutionRuntimeNodeAgent,
		SubjectID:      identity.AgentRoundID,
		RootRoundID:    identity.RootRoundID,
		RuntimeRoundID: identity.RuntimeRoundID,
		AgentRoundID:   identity.AgentRoundID,
		AgentID:        identity.AgentID,
		Name:           descriptor.Name,
		Description:    description,
		Status:         status,
		Failed:         status == protocol.ExecutionRuntimeNodeFailed,
		StartedAt:      now,
		UpdatedAt:      now,
		FinishedAt:     &now,
		Metadata:       descriptor.Metadata,
	}); err != nil {
		return err
	}
	return repository.FinishRuntimeGraphRound(
		ctx,
		identity.OwnerUserID,
		identity.SessionKey,
		identity.AgentRoundID,
		status,
		now,
	)
}

type runtimeAgentDescriptor struct {
	Name        string
	Description string
	Metadata    map[string]any
}

func runtimeAgentNodeDescriptor(actor ActorContext) runtimeAgentDescriptor {
	role := strings.TrimSpace(string(actor.Role))
	descriptor := runtimeAgentDescriptor{
		Name:        "respond",
		Description: "Handle the current conversation or task round",
		Metadata: map[string]any{
			"actor_role": role,
		},
	}
	if binding := actor.ReviewBinding; binding != nil {
		descriptor.Name = "review"
		descriptor.Description = "Review the selected Work Item submission"
		descriptor.Metadata["execution_lane"] = "review"
		descriptor.Metadata["work_item_id"] = strings.TrimSpace(binding.WorkItemID)
		descriptor.Metadata["submission_id"] = strings.TrimSpace(binding.SubmissionID)
		return descriptor
	}
	if binding := actor.WorkBinding; binding != nil {
		descriptor.Name = "work"
		descriptor.Description = "Execute the assigned Work Item"
		descriptor.Metadata["execution_lane"] = "work"
		descriptor.Metadata["work_item_id"] = strings.TrimSpace(binding.WorkItemID)
		descriptor.Metadata["assignment_id"] = strings.TrimSpace(binding.AssignmentID)
		return descriptor
	}
	if actor.Role == ExecutionActorCoordinator {
		descriptor.Name = "coordinate"
		descriptor.Description = "Plan, integrate, review, recover, or deliver as coordinator"
		descriptor.Metadata["execution_lane"] = "coordination"
	}
	return descriptor
}

type runtimeGraphIdentity struct {
	GraphID        string
	OwnerUserID    string
	SessionKey     string
	ExecutionID    string
	RootRoundID    string
	RuntimeRoundID string
	AgentRoundID   string
	AgentID        string
}

func runtimeGraphIdentityFromActor(actor ActorContext) (runtimeGraphIdentity, error) {
	value := runtimeGraphIdentity{
		OwnerUserID:    strings.TrimSpace(actor.OwnerUserID),
		SessionKey:     strings.TrimSpace(actor.SessionKey),
		ExecutionID:    strings.TrimSpace(actor.ExecutionID),
		RootRoundID:    strings.TrimSpace(actor.RootRoundID),
		RuntimeRoundID: strings.TrimSpace(actor.RuntimeRoundID),
		AgentRoundID:   strings.TrimSpace(actor.AgentRoundID),
		AgentID:        strings.TrimSpace(actor.AgentID),
	}
	if value.RuntimeRoundID == "" {
		value.RuntimeRoundID = value.AgentRoundID
	}
	if value.RootRoundID == "" {
		value.RootRoundID = value.RuntimeRoundID
	}
	if value.AgentRoundID == "" {
		value.AgentRoundID = value.RuntimeRoundID
	}
	if value.OwnerUserID == "" || value.SessionKey == "" ||
		value.RootRoundID == "" || value.RuntimeRoundID == "" ||
		value.AgentRoundID == "" || value.AgentID == "" {
		return runtimeGraphIdentity{}, domainError(
			ErrorCodeInvalidInput,
			"runtime graph requires owner, session, round and agent identity",
		)
	}
	if value.ExecutionID != "" {
		value.GraphID = "execution:" + value.ExecutionID
	} else {
		value.GraphID = "round:" + value.RootRoundID
	}
	return value, nil
}

func runtimeGraphKinds(
	kind sdkprotocol.RuntimeLifecycleNodeKind,
) (protocol.ExecutionRuntimeNodeKind, protocol.ExecutionRuntimeEdgeKind, error) {
	switch kind {
	case sdkprotocol.RuntimeLifecycleNodeTool:
		return protocol.ExecutionRuntimeNodeTool, protocol.ExecutionRuntimeEdgeInvoke, nil
	case sdkprotocol.RuntimeLifecycleNodeSubagent:
		return protocol.ExecutionRuntimeNodeSubagent, protocol.ExecutionRuntimeEdgeSpawn, nil
	default:
		return "", "", fmt.Errorf("unsupported runtime lifecycle node kind %q", kind)
	}
}

func runtimeGraphStatus(event sdkprotocol.RuntimeLifecycleEvent) protocol.ExecutionRuntimeNodeStatus {
	if event.Phase != sdkprotocol.RuntimeLifecycleFinished {
		return protocol.ExecutionRuntimeNodeRunning
	}
	switch strings.ToLower(strings.TrimSpace(event.Status)) {
	case "failed", "error", "failure":
		return protocol.ExecutionRuntimeNodeFailed
	case "cancelled", "canceled":
		return protocol.ExecutionRuntimeNodeCancelled
	case "interrupted", "stopped", "aborted":
		return protocol.ExecutionRuntimeNodeInterrupted
	default:
		if event.Failed {
			return protocol.ExecutionRuntimeNodeFailed
		}
		return protocol.ExecutionRuntimeNodeSucceeded
	}
}

func normalizeRuntimeGraphTerminalStatus(
	terminalStatus string,
	failureReason string,
) protocol.ExecutionRuntimeNodeStatus {
	switch strings.ToLower(strings.TrimSpace(terminalStatus)) {
	case "success", "succeeded", "completed", "complete", "done":
		return protocol.ExecutionRuntimeNodeSucceeded
	case "cancelled", "canceled":
		return protocol.ExecutionRuntimeNodeCancelled
	case "interrupted", "stopped", "aborted":
		return protocol.ExecutionRuntimeNodeInterrupted
	case "error", "failed", "failure":
		return protocol.ExecutionRuntimeNodeFailed
	default:
		if strings.Contains(strings.ToLower(failureReason), "interrupt") {
			return protocol.ExecutionRuntimeNodeInterrupted
		}
		if strings.TrimSpace(failureReason) != "" {
			return protocol.ExecutionRuntimeNodeFailed
		}
		return protocol.ExecutionRuntimeNodeSucceeded
	}
}

func runtimeGraphNodeID(
	identity runtimeGraphIdentity,
	kind protocol.ExecutionRuntimeNodeKind,
	subjectID string,
) string {
	return stableRuntimeGraphID(
		"runtime_node",
		identity.OwnerUserID,
		identity.SessionKey,
		identity.AgentRoundID,
		string(kind),
		strings.TrimSpace(subjectID),
	)
}

func runtimeGraphEdgeID(item protocol.ExecutionRuntimeEdgeRun) string {
	return stableRuntimeGraphID(
		"runtime_edge",
		item.GraphID,
		item.SourceNodeID,
		item.TargetNodeID,
		string(item.Kind),
	)
}

func stableRuntimeGraphID(prefix string, values ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return prefix + "_" + hex.EncodeToString(hash[:16])
}
