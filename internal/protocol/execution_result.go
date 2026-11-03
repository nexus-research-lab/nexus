// INPUT: Explicit structured delivery or review declared by a bound runtime round.
// OUTPUT: Strict result payload independent of tool transport; no caller-selected work identity.
// POS: Runtime result-hook wire contract. Only the host's exact binding selects the target.
package protocol

type ExecutionResultDeclaration struct {
	Version  int                      `json:"version"`
	RoundID  string                   `json:"round_id"`
	Delivery *ExecutionDeliveryResult `json:"delivery,omitempty"`
	Review   *ExecutionReviewResult   `json:"review,omitempty"`
}

type ExecutionDeliveryResult struct {
	ResultSummary string   `json:"result_summary"`
	ResultRefs    []string `json:"result_refs,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

type ExecutionReviewResult struct {
	Decision        WorkAcceptanceDecision          `json:"decision"`
	CriteriaResults []WorkAcceptanceCriterionResult `json:"criteria_results,omitempty"`
	Feedback        string                          `json:"feedback,omitempty"`
}
