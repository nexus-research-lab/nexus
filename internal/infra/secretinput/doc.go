// Package secretinput defines the model-visible placeholder and write-time
// leaf redaction contract for conversational configuration secrets. Read-time
// whole-object projections remain the responsibility of each owning domain.
//
// L2
// Members:
//   - input.go: placeholder validation, materialization, and tool-input redaction.
//
// Exposed: Slot, PrepareJSON, MaterializeJSON, SlotsFromToolInput,
// RedactConfigurationToolInput, IsSensitiveKey.
package secretinput
