// INPUT: Versioned template parameter definitions and explicit invocation values.
// OUTPUT: Typed parameter contract, exact workflow provenance and measured run aggregates.
// POS: Reusable semantic input and statistics only; no runtime capability or copied work result.
package protocol

type WorkGraphParameter struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
}
type WorkGraphWorkflowSource struct {
	ID         string         `json:"id" yaml:"id"`
	Version    int64          `json:"version" yaml:"version"`
	Parameters map[string]any `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}
type WorkGraphWorkflowStatistics struct {
	Version                 int64 `json:"version"`
	Runs                    int   `json:"runs"`
	Completed               int   `json:"completed"`
	ClosedWithoutCompletion int   `json:"closed_without_completion"`
	Active                  int   `json:"active"`
	Attempts                int   `json:"attempts"`
	FailedAttempts          int   `json:"failed_attempts"`
	ElapsedSamples          int   `json:"elapsed_samples"`
	ElapsedMilliseconds     int64 `json:"elapsed_milliseconds"`
}
