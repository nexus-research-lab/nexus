// INPUT: Provider mutation validation, ownership, aggregate transaction and post-commit projection failures.
// OUTPUT: Stable errors that let HTTP callers state the data effect without parsing user-facing text.
// POS: Provider service failure evidence boundary; error text remains descriptive, while errors.Is owns semantics.
package provider

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidInput means the Provider mutation was rejected before entering its write transaction.
	ErrInvalidInput = errors.New("provider input invalid")
	// ErrProviderAlreadyExists means the exact owner-scoped Provider key already exists.
	ErrProviderAlreadyExists = errors.New("provider already exists")
	// ErrProviderManagementForbidden means the caller cannot mutate the target Provider scope.
	ErrProviderManagementForbidden = errors.New("provider management forbidden")
	// ErrProviderInUse means a non-forced delete was rejected before the delete transaction.
	ErrProviderInUse = errors.New("provider still in use")
	// ErrMutationCommitted means the aggregate transaction committed but the response projection failed.
	ErrMutationCommitted = errors.New("provider mutation committed")
)

func invalidInputError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrInvalidInput, err)
}

func mutationCommittedError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrMutationCommitted, err)
}

func mutationNotAppliedError(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(ErrMutationNotApplied, err)
}
