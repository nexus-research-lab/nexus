// INPUT: Conversation 请求或 Agent round 在宿主边界确认的失败分类。
// OUTPUT: 可跨 WebSocket 投影、但不包含内部诊断详情的稳定 failure_code。
// POS: Conversation reliability 的 wire 枚举真相；重试与恢复状态仍由各领域状态机维护。
package protocol

// ConversationFailureCode 是面向产品恢复语义的稳定分类，不是内部错误文本。
type ConversationFailureCode string

const (
	ConversationFailureConnectionUnavailable ConversationFailureCode = "connection_unavailable"
	ConversationFailureDeliveryUnknown       ConversationFailureCode = "delivery_unknown"
	ConversationFailurePermissionNotSent     ConversationFailureCode = "permission_not_sent"
	ConversationFailureProviderConfiguration ConversationFailureCode = "provider_configuration"
	ConversationFailureProviderUnavailable   ConversationFailureCode = "provider_unavailable"
	ConversationFailureRequestRejected       ConversationFailureCode = "request_rejected"
	ConversationFailureRoundFailed           ConversationFailureCode = "round_failed"
	ConversationFailureSafetyRejected        ConversationFailureCode = "safety_rejected"
	ConversationFailureSessionLoadFailed     ConversationFailureCode = "session_load_failed"
	ConversationFailureUsageLimited          ConversationFailureCode = "usage_limited"
	ConversationFailureValidationFailed      ConversationFailureCode = "validation_failed"
)

// WithConversationFailureCode 为错误事件补充产品分类，不改变原有诊断 message。
func WithConversationFailureCode(event EventMessage, code ConversationFailureCode) EventMessage {
	if event.Data == nil {
		event.Data = make(map[string]any)
	}
	event.Data["failure_code"] = code
	return event
}
