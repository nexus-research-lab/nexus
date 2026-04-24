package message

import "strings"

const (
	permissionRequestTimeoutMessage       = "Permission request timeout"
	permissionChannelUnavailableMessage   = "Permission channel unavailable"
	askUserQuestionTimeoutErrorCode       = "permission_request_timeout"
	askUserQuestionChannelUnavailableCode = "permission_channel_unavailable"
)

// inferPermissionErrorCode 根据工具类型与错误文本推导结构化错误码。
// 对齐 Python 后端 permission_error_codes.infer_permission_error_code 逻辑。
func inferPermissionErrorCode(toolName string, message string) string {
	normalizedMessage := strings.TrimSpace(message)
	if toolName != "AskUserQuestion" || normalizedMessage == "" {
		return ""
	}
	if normalizedMessage == permissionRequestTimeoutMessage {
		return askUserQuestionTimeoutErrorCode
	}
	if normalizedMessage == permissionChannelUnavailableMessage {
		return askUserQuestionChannelUnavailableCode
	}
	return ""
}
