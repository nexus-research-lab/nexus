package shared

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
)

const (
	// DesktopSessionTokenHeader 是桌面 shell 注入到 HTTP API 请求里的本地会话凭据。
	DesktopSessionTokenHeader = "X-Nexus-Desktop-Token"

	// DesktopSessionTokenCookie 是 WKWebView WebSocket 握手使用的本地会话凭据。
	DesktopSessionTokenCookie = "nexus_desktop_token"

	// DesktopWebSocketSubprotocol 是桌面 WebSocket 握手协商出的非敏感子协议。
	DesktopWebSocketSubprotocol = "nexus.desktop.v1"

	// DesktopSessionTokenProtocolPrefix 是 WebSocket 握手使用的子协议 token 前缀。
	DesktopSessionTokenProtocolPrefix = "nexus.desktop.token."
)

type desktopSessionTokenValidation struct {
	valid  bool
	source string
	reason string
}

// responseRecorder 负责在不破坏 websocket/hijack 能力的前提下记录状态码和字节数。
type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func newResponseRecorder(writer http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: writer,
		status:         http.StatusOK,
	}
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(payload []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	size, err := r.ResponseWriter.Write(payload)
	r.bytesWritten += size
	return size, err
}

func (r *responseRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	if source, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		size, err := source.ReadFrom(reader)
		r.bytesWritten += int(size)
		return size, err
	}
	return io.Copy(r, reader)
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *responseRecorder) Push(target string, options *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (r *responseRecorder) HeaderWritten() bool {
	return r.wroteHeader
}

// RequestContextMiddleware 注入 request_id 和 request logger。
func RequestContextMiddleware(baseLogger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requestID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
			if requestID == "" {
				requestID = generateRequestID()
			}
			writer.Header().Set("X-Request-ID", requestID)

			requestLogger := baseLogger.With("request_id", requestID)
			ctx := logx.WithRequestID(request.Context(), requestID)
			ctx = logx.WithLogger(ctx, requestLogger)
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

// AccessLogMiddleware 记录请求访问日志。
func AccessLogMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			startedAt := time.Now()
			recorder := newResponseRecorder(writer)
			next.ServeHTTP(recorder, request)

			requestLogger := logx.FromContext(request.Context())
			duration := time.Since(startedAt)
			fields := []any{
				"method", request.Method,
				"status", recorder.status,
				"duration_ms", duration.Milliseconds(),
				"bytes", recorder.bytesWritten,
				"remote_ip", ClientIP(request),
				"path", request.URL.Path,
			}
			if rawQuery := strings.TrimSpace(request.URL.RawQuery); rawQuery != "" {
				fields = append(fields, "query", sanitizeQueryForLog(rawQuery))
			}

			switch accessLogLevel(request, recorder.status, duration) {
			case slog.LevelError:
				requestLogger.Error("HTTP 请求完成", fields...)
			case slog.LevelWarn:
				requestLogger.Warn("HTTP 请求完成", fields...)
			case slog.LevelInfo:
				requestLogger.Info("HTTP 请求完成", fields...)
			default:
				requestLogger.Debug("HTTP 请求完成", fields...)
			}
		})
	}
}

func accessLogLevel(request *http.Request, status int, duration time.Duration) slog.Level {
	if status >= http.StatusInternalServerError {
		return slog.LevelError
	}
	if status >= http.StatusBadRequest {
		return slog.LevelWarn
	}
	if status == http.StatusSwitchingProtocols {
		return slog.LevelDebug
	}
	if duration >= time.Second {
		return slog.LevelInfo
	}
	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// RecoverMiddleware 捕获 panic 并返回标准错误。
func RecoverMiddleware(api *API) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					requestLogger := logx.FromContext(request.Context())
					requestLogger.Error("HTTP 请求 panic",
						"method", request.Method,
						"path", request.URL.Path,
						"remote_ip", ClientIP(request),
						"panic", fmt.Sprint(recovered),
						"stack", string(debug.Stack()),
					)

					if recorder, ok := writer.(*responseRecorder); ok && recorder.HeaderWritten() {
						return
					}
					api.WriteFailure(writer, http.StatusInternalServerError, "服务内部错误")
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}

// DesktopSessionTokenMiddleware 校验桌面 App 本地 API 面的一次性会话 token。
func DesktopSessionTokenMiddleware(api *API, token string, apiPrefix string) func(http.Handler) http.Handler {
	expectedToken := strings.TrimSpace(token)
	normalizedAPIPrefix := normalizeAPIPrefix(apiPrefix)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if expectedToken == "" || desktopSessionTokenBypass(request, normalizedAPIPrefix) {
				next.ServeHTTP(writer, request)
				return
			}
			validation := validateDesktopSessionToken(request, expectedToken)
			if !validation.valid {
				logx.Resolve(request.Context(), api.BaseLogger()).Warn(
					"桌面会话 token 校验失败",
					"reason", validation.reason,
					"token_source", validation.source,
					"has_header", strings.TrimSpace(request.Header.Get(DesktopSessionTokenHeader)) != "",
					"has_protocol_token", desktopSessionTokenFromProtocolHeader(request.Header.Get("Sec-WebSocket-Protocol")) != "",
					"has_cookie", desktopSessionTokenFromCookie(request) != "",
					"method", request.Method,
					"path", request.URL.Path,
					"user_agent", request.UserAgent(),
				)
				api.WriteFailure(writer, http.StatusUnauthorized, "桌面会话 token 无效")
				return
			}
			next.ServeHTTP(writer, request)
		})
	}
}

// AuthMiddleware 把认证状态写入请求上下文。
func AuthMiddleware(api *API, auth *authsvc.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if auth == nil {
				next.ServeHTTP(writer, request)
				return
			}

			principal, state, err := auth.InspectRequest(request.Context(), request)
			if err != nil {
				api.WriteFailure(writer, http.StatusInternalServerError, err.Error())
				return
			}

			ctx := authsvc.WithState(request.Context(), state)
			ctx = authsvc.WithPrincipal(ctx, principal)
			if PublicAuthRoute(request) || !state.AuthRequired {
				next.ServeHTTP(writer, request.WithContext(ctx))
				return
			}
			if principal == nil {
				api.WriteFailure(writer, http.StatusUnauthorized, "未登录或登录状态已过期")
				return
			}
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

func normalizeAPIPrefix(prefix string) string {
	value := strings.TrimSpace(prefix)
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if len(value) > 1 {
		value = strings.TrimRight(value, "/")
	}
	return value
}

func desktopSessionTokenBypass(request *http.Request, apiPrefix string) bool {
	if request == nil {
		return true
	}
	if request.Method == http.MethodOptions {
		return true
	}
	path := strings.TrimSpace(request.URL.Path)
	if path != apiPrefix && !strings.HasPrefix(path, apiPrefix+"/") {
		return true
	}
	switch path {
	case apiPrefix + "/health",
		apiPrefix + "/system/version":
		return true
	}
	if strings.HasPrefix(path, apiPrefix+"/internal/") {
		return true
	}
	return false
}

func validDesktopSessionToken(request *http.Request, expectedToken string) bool {
	return validateDesktopSessionToken(request, expectedToken).valid
}

func validateDesktopSessionToken(request *http.Request, expectedToken string) desktopSessionTokenValidation {
	providedToken := strings.TrimSpace(request.Header.Get(DesktopSessionTokenHeader))
	if providedToken != "" {
		valid := subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) == 1
		return desktopSessionTokenValidation{
			valid:  valid,
			source: "header",
			reason: validationReason(valid, "header_mismatch"),
		}
	}
	providedToken = desktopSessionTokenFromProtocolHeader(request.Header.Get("Sec-WebSocket-Protocol"))
	if providedToken != "" {
		valid := subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) == 1
		return desktopSessionTokenValidation{
			valid:  valid,
			source: "protocol",
			reason: validationReason(valid, "protocol_mismatch"),
		}
	}
	providedToken = desktopSessionTokenFromCookie(request)
	if providedToken != "" {
		valid := subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) == 1
		return desktopSessionTokenValidation{
			valid:  valid,
			source: "cookie",
			reason: validationReason(valid, "cookie_mismatch"),
		}
	}
	return desktopSessionTokenValidation{
		valid:  false,
		source: "none",
		reason: "missing",
	}
}

func validationReason(valid bool, mismatchReason string) string {
	if valid {
		return "ok"
	}
	return mismatchReason
}

func desktopSessionTokenFromProtocolHeader(rawHeader string) string {
	for _, part := range strings.Split(rawHeader, ",") {
		value := strings.TrimSpace(part)
		if strings.HasPrefix(value, DesktopSessionTokenProtocolPrefix) {
			return strings.TrimPrefix(value, DesktopSessionTokenProtocolPrefix)
		}
	}
	return ""
}

func desktopSessionTokenFromCookie(request *http.Request) string {
	cookie, err := request.Cookie(DesktopSessionTokenCookie)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

// ClientIP 返回请求来源 IP。
func ClientIP(request *http.Request) string {
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := strings.TrimSpace(request.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(request.RemoteAddr)
}

// sanitizeQueryForLog 脱敏 URL 查询字符串中的 token 类参数，防止凭证写入访问日志。
func sanitizeQueryForLog(rawQuery string) string {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "[unparseable]"
	}
	for _, key := range []string{"access_token", "token", "api_key", "secret", "password"} {
		if values.Has(key) {
			values.Set(key, "[redacted]")
		}
	}
	return values.Encode()
}

func generateRequestID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
