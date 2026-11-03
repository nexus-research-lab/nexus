// INPUT: Workflow consumer ports and the production SQL repository.
// OUTPUT: Compile-time proof that production always uses durable Draft semantics.
// POS: Prevents silently falling back to process-local test substitute state.
package workgraphworkflow

import workflowstore "github.com/nexus-research-lab/nexus/internal/storage/workgraphworkflow"

var (
	_ Repository      = (*workflowstore.Repository)(nil)
	_ DraftRepository = (*workflowstore.Repository)(nil)
)
