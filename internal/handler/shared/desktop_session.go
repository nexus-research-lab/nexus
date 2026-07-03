package shared

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
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
	if request.Method == http.MethodPost && path == apiPrefix+"/connectors/oauth/callback" {
		return true
	}
	if strings.HasPrefix(path, apiPrefix+"/internal/") {
		return true
	}
	return false
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
