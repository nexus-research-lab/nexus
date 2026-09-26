package contract

// RetryableIngressError 仅表示尚未进入命令或运行时派发，可沿同一事件身份重试。
// 数据库写入、配对和路由准备可以已发生；调用方不得给未知执行结果加此标记。
type RetryableIngressError struct {
	Err error
}

func (e *RetryableIngressError) Error() string { return e.Err.Error() }
func (e *RetryableIngressError) Unwrap() error { return e.Err }
