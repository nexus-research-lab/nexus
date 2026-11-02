// INPUT: HTTP 中间件确认的一次传输 request ID。
// OUTPUT: 仅供日志、失败响应与下游透传关联的 context 存取器（存储归 infra/logx）。
// POS: HTTP 诊断身份边界；不得被业务服务用作授权、路由、缓存或幂等身份。
package shared

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
)

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
	return logx.WithRequestID(ctx, normalizeDiagnosticRequestID(requestID))
}

// requestID 返回当前 HTTP 传输尝试的诊断 ID，仅供 shared 写出与测试使用。
func requestID(ctx context.Context) string {
	return logx.RequestID(ctx)
}
