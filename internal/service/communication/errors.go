// INPUT: 平台通讯在任何副作用前确认的用户输入问题。
// OUTPUT: 可被 HTTP 等适配层稳定识别、同时保留原兼容文案的 typed error。
// POS: 通讯领域的提交前失败证据；传输层不得再从 error 文本猜测结果。
package communication

// InputError 表示通讯请求在进入 Room 创建或消息持久化前已被拒绝。
type InputError struct {
	detail string
}

func (e *InputError) Error() string {
	if e == nil {
		return "通讯请求无效"
	}
	return e.detail
}

func newInputError(detail string) error {
	return &InputError{detail: detail}
}
