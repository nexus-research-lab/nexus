// INPUT: Stable Provider service sentinels, unclassified text and conditional-write ETags.
// OUTPUT: Legacy HTTP statuses plus evidence-based FailureCore effect/code and strict version parsing.
// POS: Provider handler recovery-contract regression tests; no business effect is inferred from error text.
package provider

import (
	"errors"
	"net/http"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestProviderMutationFailureUsesStableEvidence(t *testing.T) {
	tests := []struct {
		name           string
		cause          error
		preconditioned bool
		status         int
		code           string
		effect         protocol.FailureEffect
	}{
		{
			name:   "invalid input",
			cause:  providercfg.ErrInvalidInput,
			status: http.StatusBadRequest,
			code:   "provider.update_invalid",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "duplicate",
			cause:  providercfg.ErrProviderAlreadyExists,
			status: http.StatusBadRequest,
			code:   "provider.already_exists",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "not found",
			cause:  providercfg.ErrProviderNotFound,
			status: http.StatusNotFound,
			code:   "provider.not_found",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "forbidden",
			cause:  providercfg.ErrProviderManagementForbidden,
			status: http.StatusForbidden,
			code:   "provider.management_forbidden",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "in use",
			cause:  providercfg.ErrProviderInUse,
			status: http.StatusBadRequest,
			code:   "provider.in_use",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "legacy version conflict",
			cause:  providercfg.ErrConfigurationVersionConflict,
			status: http.StatusBadRequest,
			code:   "provider.version_conflict",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:           "conditional version conflict",
			cause:          providercfg.ErrConfigurationVersionConflict,
			preconditioned: true,
			status:         http.StatusPreconditionFailed,
			code:           "provider.version_conflict",
			effect:         protocol.FailureEffectNotApplied,
		},
		{
			name:   "post-commit projection",
			cause:  errors.Join(providercfg.ErrMutationCommitted, errors.New("projection failed")),
			status: http.StatusBadRequest,
			code:   "provider.update_committed",
			effect: protocol.FailureEffectCommitted,
		},
		{
			name:   "confirmed rollback",
			cause:  errors.Join(providercfg.ErrMutationNotApplied, errors.New("transaction rolled back")),
			status: http.StatusBadRequest,
			code:   "provider.update_not_applied",
			effect: protocol.FailureEffectNotApplied,
		},
		{
			name:   "unclassified",
			cause:  errors.New("database connection closed"),
			status: http.StatusBadRequest,
			code:   "provider.update_result_unknown",
			effect: protocol.FailureEffectUnknown,
		},
		{
			name:   "misleading text is not evidence",
			cause:  errors.New("不存在，只有管理员，validation failed"),
			status: http.StatusBadRequest,
			code:   "provider.update_result_unknown",
			effect: protocol.FailureEffectUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, failure := providerMutationFailure(
				"update",
				tt.cause,
				tt.preconditioned,
			)
			if status != tt.status {
				t.Fatalf("status = %d, want %d", status, tt.status)
			}
			if failure.Code != tt.code || failure.Effect != tt.effect {
				t.Fatalf("failure = code %q effect %q", failure.Code, failure.Effect)
			}
		})
	}
}

func TestProviderImportPreviewFailureIsReadOnly(t *testing.T) {
	failure := providerImportPreviewFailure(errors.New("config directory unavailable"))
	if failure.Code != "provider.preview_import_failed" ||
		failure.Effect != protocol.FailureEffectNotApplicable {
		t.Fatalf("preview failure = code %q effect %q", failure.Code, failure.Effect)
	}
}

func TestProviderImportPreviewInvalidRequestRemainsReadOnly(t *testing.T) {
	failure := providerPreviewRequestInvalidFailure()
	if failure.Code != "provider.preview_import_request_invalid" ||
		failure.Category != protocol.FailureCategoryValidation ||
		failure.Effect != protocol.FailureEffectNotApplicable {
		t.Fatalf(
			"preview request failure = code %q category %q effect %q",
			failure.Code,
			failure.Category,
			failure.Effect,
		)
	}
}

func TestParseProviderIfMatchAcceptsOnlyExactStrongVersion(t *testing.T) {
	tests := []struct {
		value   string
		version int64
		valid   bool
	}{
		{value: "", valid: true},
		{value: `"provider-7"`, version: 7, valid: true},
		{value: `W/"provider-7"`},
		{value: `"provider-7", "provider-8"`},
		{value: `"other-7"`},
		{value: `"provider-0"`},
		{value: `provider-7`},
	}
	for _, tt := range tests {
		version, err := parseProviderIfMatch(tt.value)
		if !tt.valid {
			if err == nil {
				t.Fatalf("parseProviderIfMatch(%q) unexpectedly succeeded", tt.value)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseProviderIfMatch(%q): %v", tt.value, err)
		}
		if tt.value == "" {
			if version != nil {
				t.Fatalf("empty If-Match version = %v", *version)
			}
			continue
		}
		if version == nil || *version != tt.version {
			t.Fatalf("parseProviderIfMatch(%q) = %v, want %d", tt.value, version, tt.version)
		}
	}
}
