// INPUT: Orchestration consumer interfaces and the production SQL repository.
// OUTPUT: Build-time failure when a persistence capability changes or disappears.
// POS: Consumer-side conformance checks; storage never imports service interfaces.
package orchestration

import orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"

var (
	_ CompletionAuditRepository            = (*orchestrationstore.Repository)(nil)
	_ GoalConfirmationRepository           = (*orchestrationstore.Repository)(nil)
	_ PlanMaterializationReceiptReader     = (*orchestrationstore.Repository)(nil)
	_ PlanProposalRepository               = (*orchestrationstore.Repository)(nil)
	_ Repository                           = (*orchestrationstore.Repository)(nil)
	_ assignmentBatchRepository            = (*orchestrationstore.Repository)(nil)
	_ cancellationDispatchOutboxRepository = (*orchestrationstore.Repository)(nil)
	_ dispatchDeadlineRepository           = (*orchestrationstore.Repository)(nil)
	_ dispatchOutboxRepository             = (*orchestrationstore.Repository)(nil)
	_ goalRevisionExecutionRepository      = (*orchestrationstore.Repository)(nil)
	_ managedExecutionHistoryRepository    = (*orchestrationstore.Repository)(nil)
	_ managedExecutionViewRepository       = (*orchestrationstore.Repository)(nil)
	_ persistenceEvidenceRepository        = (*orchestrationstore.Repository)(nil)
	_ recoveryDeadlineRepository           = (*orchestrationstore.Repository)(nil)
	_ reviewDispatchOutboxRepository       = (*orchestrationstore.Repository)(nil)
	_ rootRecoveryRepository               = (*orchestrationstore.Repository)(nil)
	_ runtimeGraphRepository               = (*orchestrationstore.Repository)(nil)
	_ subagentDeadlineRepository           = (*orchestrationstore.Repository)(nil)
	_ subagentReconciliationRepository     = (*orchestrationstore.Repository)(nil)
	_ workGraphRuntimeRepository           = (*orchestrationstore.Repository)(nil)
	_ workGraphStateRepository             = (*orchestrationstore.Repository)(nil)
)
