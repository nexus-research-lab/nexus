// INPUT: Channel 控制写入在事务、补偿或写后投影阶段返回的原始错误。
// OUTPUT: 保留 errors.Is/errors.As 的最小数据影响证据，供 HTTP 边界安全投影。
// POS: Channel 领域的失败阶段标记；不承载用户文案、HTTP 状态或重试策略。
package channels

import "errors"

// ControlMutationEffect 是 Channel 领域能够证明的持久化结果。
//
// 它刻意不复用 HTTP FailureCore，避免 service 依赖传输层；Handler 负责把
// 这里的领域证据映射为开放 wire 值。
type ControlMutationEffect string

const (
	ControlMutationNotApplied ControlMutationEffect = "not_applied"
	ControlMutationCommitted  ControlMutationEffect = "committed"
	ControlMutationUnknown    ControlMutationEffect = "unknown"
)

var (
	// ErrChannelControlInvalid 表示请求在进入持久化阶段前即可确定无效。
	ErrChannelControlInvalid = errors.New("channel control request is invalid")
	// ErrChannelConfigRequired 表示登录等动作要求已有 Channel 配置。
	ErrChannelConfigRequired = errors.New("channel configuration is required")
	// ErrChannelCredentialStoreUnavailable 表示宿主无法安全读写 Channel 凭据。
	ErrChannelCredentialStoreUnavailable = errors.New("channel credential store is unavailable")
)

// ControlMutationError 为失败附加领域已经证明的数据影响；Cause 保持原错误链。
type ControlMutationError struct {
	effect ControlMutationEffect
	cause  error
}

func (e *ControlMutationError) Error() string {
	if e == nil || e.cause == nil {
		return "channel control mutation failed"
	}
	return e.cause.Error()
}

func (e *ControlMutationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// ChannelControlMutationEffect 返回最外层 Channel 写入阶段证据。
func ChannelControlMutationEffect(err error) (ControlMutationEffect, bool) {
	var mutationErr *ControlMutationError
	if !errors.As(err, &mutationErr) || mutationErr == nil {
		return "", false
	}
	switch mutationErr.effect {
	case ControlMutationNotApplied, ControlMutationCommitted, ControlMutationUnknown:
		return mutationErr.effect, true
	default:
		return "", false
	}
}

func channelControlMutationFailure(effect ControlMutationEffect, cause error) error {
	if cause == nil {
		return nil
	}
	return &ControlMutationError{effect: effect, cause: cause}
}

func invalidChannelControl(cause error) error {
	if cause == nil {
		cause = ErrChannelControlInvalid
	}
	return channelControlMutationFailure(
		ControlMutationNotApplied,
		&channelClassifiedError{marker: ErrChannelControlInvalid, cause: cause},
	)
}

type channelClassifiedError struct {
	marker error
	cause  error
}

func (e *channelClassifiedError) Error() string {
	if e == nil || e.cause == nil {
		return "channel control failed"
	}
	return e.cause.Error()
}

func (e *channelClassifiedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *channelClassifiedError) Is(target error) bool {
	return e != nil && (target == e.marker || errors.Is(e.cause, target))
}

func classifyChannelControlError(marker error, cause error) error {
	if cause == nil {
		cause = marker
	}
	return &channelClassifiedError{marker: marker, cause: cause}
}
