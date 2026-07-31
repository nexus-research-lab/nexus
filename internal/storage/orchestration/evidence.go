// INPUT: runtime/scheduler 已观察的持续性证据、Execution CAS 与审计身份。
// OUTPUT: metadata 中不可由模型伪造的 evidence flag 和同事务领域事件。
// POS: adaptive Goal promotion 的持久证据写入边界。
package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RecordEvidence 把一个后端观察到的持续性事实幂等写入 Execution。
func (r *Repository) RecordEvidence(
	ctx context.Context,
	command RecordEvidenceCommand,
) (*protocol.ExecutionSnapshot, error) {
	key := strings.TrimSpace(command.MetadataKey)
	if key == "" {
		return nil, fmt.Errorf("%w: evidence metadata key is required", ErrInvariant)
	}
	command.Meta.Payload = map[string]any{"evidence_key": key}
	mutation, err := r.beginMutation(
		ctx,
		command.ExecutionID,
		command.ExpectedExecutionVersion,
		command.Meta,
		protocol.ExecutionEventEvidenceRecorded,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	execution, err := r.getExecution(ctx, mutation.tx, command.ExecutionID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if execution == nil {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: execution was not found", ErrInvariant)
	}
	metadata := execution.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata[key] = true
	metadataJSON, err := marshalMap(metadata)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE executions
SET metadata_json = `+r.jsonBind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND version = `+r.bind(3),
		metadataJSON,
		strings.TrimSpace(command.ExecutionID),
		command.ExpectedExecutionVersion+1,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityExecution,
		EntityID:      strings.TrimSpace(command.ExecutionID),
		EntityVersion: command.ExpectedExecutionVersion + 1,
	})
}
