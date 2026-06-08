package memory

import (
	"errors"
	"fmt"
)

// ClientError 表示由调用方输入导致的可预期错误。
type ClientError struct {
	message string
}

func (e *ClientError) Error() string {
	return e.message
}

func newClientError(format string, args ...any) error {
	return &ClientError{message: fmt.Sprintf(format, args...)}
}

// IsClientError 判断错误是否应作为 4xx 返回给调用方。
func IsClientError(err error) bool {
	var target *ClientError
	return errors.As(err, &target)
}
