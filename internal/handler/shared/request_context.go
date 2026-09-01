// INPUT: HTTP 中间件确认的一次传输 request ID。
// OUTPUT: 仅供日志和失败响应关联的 context 存取器。
// POS: HTTP 诊断身份边界；不得被业务服务用作授权、路由、缓存或幂等身份。
package shared

import (
	"context"
	"strings"
)

type requestIDContextKey struct{}

const maxDiagnosticRequestIDLength = 128

func normalizeDiagnosticRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxDiagnosticRequestIDLength {
		return ""
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') {
			continue
		}
		switch character {
		case '-', '_', '.', ':':
			continue
		default:
			return ""
		}
	}
	return value
}

func withRequestID(ctx context.Context, requestID string) context.Context {
	requestID = normalizeDiagnosticRequestID(requestID)
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// requestID 返回当前 HTTP 传输尝试的诊断 ID，仅供 shared 写出与测试使用。
func requestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return strings.TrimSpace(requestID)
}
