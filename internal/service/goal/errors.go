package goal

import "errors"

var (
	ErrGoalDisabled     = errors.New("goal feature disabled")
	ErrGoalNotFound     = errors.New("goal not found")
	ErrGoalConflict     = errors.New("current goal already exists")
	ErrGoalInvalidInput = errors.New("goal invalid input")
	ErrGoalInvalidState = errors.New("goal invalid state")
	ErrGoalVersionStale = errors.New("goal version stale")
)

type goalInvalidInputError struct {
	message string
}

func (err goalInvalidInputError) Error() string {
	return err.message
}

func (err goalInvalidInputError) Is(target error) bool {
	return target == ErrGoalInvalidInput
}

func newGoalInvalidInputError(message string) error {
	if message == "" {
		return ErrGoalInvalidInput
	}
	return goalInvalidInputError{message: message}
}

type goalNotFoundError struct {
	message string
}

func (err goalNotFoundError) Error() string {
	return err.message
}

func (err goalNotFoundError) Is(target error) bool {
	return target == ErrGoalNotFound
}

func newGoalNotFoundError(message string) error {
	if message == "" {
		return ErrGoalNotFound
	}
	return goalNotFoundError{message: message}
}
