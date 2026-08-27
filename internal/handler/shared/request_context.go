// INPUT: HTTP 中间件确认的一次传输 request ID。
// OUTPUT: 仅供日志和失败响应关联的 context 存取器。
// POS: HTTP 诊断身份边界；不得被业务服务用作授权、路由、缓存或幂等身份。
package shared

import (
	"context"
	"strings"
)

type requestIDContextKey struct{}

func withRequestID(ctx context.Context, requestID string) context.Context {
	requestID = strings.TrimSpace(requestID)
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
