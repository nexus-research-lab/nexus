package auth

import (
	"errors"
	"net"
	"net/http"
	"strings"
)

func normalizeUserRole(role string) (string, error) {
	switch strings.TrimSpace(role) {
	case "", RoleMember:
		return RoleMember, nil
	case RoleAdmin:
		return RoleAdmin, nil
	case RoleOwner:
		return RoleOwner, nil
	default:
		return "", errors.New("role 仅支持 owner、admin、member")
	}
}

func normalizeUsername(username string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(username))
	if normalized == "" {
		return "", errors.New("用户名不能为空")
	}
	if len(normalized) < 3 || len(normalized) > 64 {
		return "", errors.New("用户名长度必须在 3 到 64 个字符之间")
	}
	for _, item := range normalized {
		if (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9') || item == '-' || item == '_' || item == '.' {
			continue
		}
		return "", errors.New("用户名只能包含小写字母、数字、点、横线和下划线")
	}
	return normalized, nil
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return errors.New("密码不能为空")
	}
	if len(password) < 8 {
		return errors.New("密码长度至少需要 8 个字符")
	}
	return nil
}

func normalizeAvatar(avatar string) (string, error) {
	normalized := strings.TrimSpace(avatar)
	if len(normalized) > 255 {
		return "", errors.New("头像标识不能超过 255 个字符")
	}
	return normalized, nil
}

func stringPointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

// ResolveClientIP 尝试从请求中提取真实客户端 IP。
func ResolveClientIP(request *http.Request) string {
	if request == nil {
		return ""
	}
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwarded != "" {
		value, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(value)
	}
	if realIP := strings.TrimSpace(request.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr))
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(request.RemoteAddr)
}
