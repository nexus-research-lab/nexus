package shared

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// API 提供处理器共享的 HTTP 响应与上下文辅助能力。
type API struct {
	logger *slog.Logger
}

// NewAPI 创建共享 API 辅助器。
func NewAPI(logger *slog.Logger) *API {
	return &API{logger: logger}
}

// BaseLogger 返回处理器基础 logger。
func (a *API) BaseLogger() *slog.Logger {
	if a != nil && a.logger != nil {
		return a.logger
	}
	return logx.NewDiscardLogger()
}

// HandleNotImplemented 返回占位路由 handler。
func (a *API) HandleNotImplemented(group string) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		a.WriteJSON(writer, http.StatusNotImplemented, map[string]any{
			"code": 1,
			"msg":  "not_implemented",
			"data": map[string]any{
				"group": group,
				"path":  request.URL.Path,
			},
		})
	}
}

// WriteJSON 写入原始 JSON 响应。
func (a *API) WriteJSON(writer http.ResponseWriter, status int, payload map[string]any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

// WriteSuccess 写入成功响应。
func (a *API) WriteSuccess(writer http.ResponseWriter, data any) {
	a.WriteJSON(writer, http.StatusOK, map[string]any{
		"code":    "0000",
		"message": "success",
		"success": true,
		"data":    data,
	})
}

// WriteFailure 写入失败响应。
func (a *API) WriteFailure(writer http.ResponseWriter, status int, detail string) {
	clientDetail := strings.TrimSpace(detail)
	if clientDetail != "" {
		a.BaseLogger().Warn("HTTP 请求失败", "status", status, "detail", clientDetail)
	}
	clientDetail = GatewayClientErrorDetail(status, clientDetail)
	a.WriteJSON(writer, status, map[string]any{
		"code":    FormatStatusCode(status),
		"message": "failed",
		"success": false,
		"data": map[string]any{
			"detail": clientDetail,
		},
	})
}

// BindJSON 解析 JSON 请求体。
func (a *API) BindJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	return a.bindJSONWithOptions(writer, request, target, false)
}

// BindJSONAllowEmpty 解析可为空的 JSON 请求体。
func (a *API) BindJSONAllowEmpty(writer http.ResponseWriter, request *http.Request, target any) bool {
	return a.bindJSONWithOptions(writer, request, target, true)
}

func (a *API) bindJSONWithOptions(
	writer http.ResponseWriter,
	request *http.Request,
	target any,
	allowEmpty bool,
) bool {
	if err := DecodeJSONBody(request.Body, target, allowEmpty); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return true
		}
		a.WriteFailure(writer, http.StatusBadRequest, "请求参数错误")
		return false
	}
	return true
}

// DecodeJSONBody 解析并校验单个 JSON 顶层值。
func DecodeJSONBody(body io.Reader, target any, allowEmpty bool) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return io.EOF
		}
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
	return errors.New("json body must contain a single top-level value")
}

// GatewayClientErrorDetail 规范化客户端可见错误文案。
func GatewayClientErrorDetail(status int, detail string) string {
	switch status {
	case http.StatusBadRequest:
		if IsClientMessageText(detail) {
			return detail
		}
		return "请求参数错误"
	case http.StatusUnauthorized:
		return "未授权"
	case http.StatusForbidden:
		return "禁止访问"
	case http.StatusNotFound:
		return "资源不存在"
	case http.StatusConflict:
		return "请求冲突"
	case http.StatusUnprocessableEntity:
		if IsClientMessageText(detail) {
			return detail
		}
		return "请求无效"
	default:
		if status >= http.StatusInternalServerError {
			return "服务内部错误"
		}
		if IsClientMessageText(detail) {
			return detail
		}
		return "请求失败"
	}
}

// FormatStatusCode 格式化 HTTP 状态码。
func FormatStatusCode(status int) string {
	return strings.TrimSpace(strconv.Itoa(status))
}

// ErrString 返回错误文本。
func ErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// IsClientMessageError 判断错误是否适合直接返回给客户端。
func IsClientMessageError(err error) bool {
	if err == nil {
		return false
	}
	return IsClientMessageText(err.Error())
}

// IsClientMessageText 判断文本是否属于客户端可理解错误。
func IsClientMessageText(message string) bool {
	return strings.Contains(message, "不能为空") ||
		strings.Contains(message, "不一致") ||
		strings.Contains(message, "不正确") ||
		strings.Contains(message, "已存在") ||
		strings.Contains(message, "至少") ||
		strings.Contains(message, "不支持") ||
		strings.Contains(message, "不能作为") ||
		strings.Contains(message, "不能超过") ||
		strings.Contains(message, " is required") ||
		strings.Contains(message, " must be ") ||
		strings.Contains(message, "正在运行中")
}

// IsStructuredSessionKeyError 判断错误是否为结构化 session key 错误。
func IsStructuredSessionKeyError(err error) bool {
	if err == nil {
		return false
	}
	var target protocol.StructuredSessionKeyError
	return errors.As(err, &target)
}

// StringValue 读取 map[string]any 中的字符串值。
func StringValue(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
}

// BoolValue 读取 map[string]any 中的布尔值。
func BoolValue(value any) (bool, bool) {
	typed, ok := value.(bool)
	if ok {
		return typed, true
	}
	return false, false
}

// Int64Value 读取 map[string]any 中的整数值。
func Int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

// PublicAuthRoute 判断是否为无需认证的公开路由。
func PublicAuthRoute(request *http.Request) bool {
	if request == nil {
		return true
	}
	if request.Method == http.MethodOptions {
		return true
	}
	path := strings.TrimSpace(request.URL.Path)
	switch path {
	case "/nexus/v1/health",
		"/nexus/v1/runtime/options",
		"/nexus/v1/auth/status",
		"/nexus/v1/auth/login",
		"/nexus/v1/auth/logout":
		return true
	default:
		return false
	}
}
