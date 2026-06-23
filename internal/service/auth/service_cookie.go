package auth

import (
	"cmp"
	"net/http"
	"strings"
	"time"
)

// CookieName 返回认证 Cookie 名称。
func (s *Service) CookieName() string {
	return s.cookieName()
}

// CookiePath 返回认证 Cookie 作用路径。
func (s *Service) CookiePath() string {
	return s.cookiePath()
}

// CookieSecure 返回认证 Cookie 的 secure 配置。
func (s *Service) CookieSecure() bool {
	return s.config.AuthCookieSecure
}

// CookieSameSite 返回认证 Cookie 的 SameSite 配置。
func (s *Service) CookieSameSite() http.SameSite {
	switch strings.ToLower(strings.TrimSpace(s.config.AuthCookieSameSite)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// SessionMaxAge 返回认证 Cookie 过期秒数。
func (s *Service) SessionMaxAge() int {
	return int(s.sessionTTL().Seconds())
}

func (s *Service) sessionTTL() time.Duration {
	hours := s.config.AuthSessionTTLHours
	if hours <= 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

func (s *Service) cookieName() string {
	return cmp.Or(strings.TrimSpace(s.config.AuthSessionCookieName), "nexus_session")
}

func (s *Service) cookiePath() string {
	return cmp.Or(strings.TrimSpace(s.config.APIPrefix), "/")
}
