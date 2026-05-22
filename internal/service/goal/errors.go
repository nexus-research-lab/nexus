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
