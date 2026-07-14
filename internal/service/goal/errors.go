// INPUT: Goal 生命周期、版本与 objective revision 冲突。
// OUTPUT: 跨调用方稳定的错误分类。
// POS: Goal 服务错误语义的唯一真相源。
package goal

import "errors"

var (
	ErrGoalDisabled      = errors.New("goal feature disabled")
	ErrGoalNotFound      = errors.New("goal not found")
	ErrGoalConflict      = errors.New("current goal already exists")
	ErrGoalInvalidInput  = errors.New("goal invalid input")
	ErrGoalInvalidState  = errors.New("goal invalid state")
	ErrGoalVersionStale  = errors.New("goal version stale")
	ErrGoalRevisionStale = errors.New("goal objective changed after this round started; this round cannot update the retargeted goal")
)

var expectedMutationErrors = []error{
	ErrGoalDisabled,
	ErrGoalNotFound,
	ErrGoalInvalidState,
	ErrGoalVersionStale,
	ErrGoalRevisionStale,
}

// IsExpectedMutationError 识别并发推进和功能关闭产生的可预期结果，调用方无需重复维护哨兵集合。
func IsExpectedMutationError(err error) bool {
	for _, target := range expectedMutationErrors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

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
