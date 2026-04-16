// =====================================================
// @File   ：config.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 承载 Go 服务运行时配置。
type Config struct {
	Host                           string
	Port                           int
	Debug                          bool
	ProjectName                    string
	LogLevel                       string
	LogFormat                      string
	LogPath                        string
	LogStdout                      bool
	LogFileEnabled                 bool
	LogRotateDaily                 bool
	LogMaxSizeMB                   int
	LogMaxAgeDays                  int
	LogMaxBackups                  int
	LogCompress                    bool
	APIPrefix                      string
	WebSocketPath                  string
	DefaultAgentID                 string
	WorkspacePath                  string
	CacheFileDir                   string
	NpmRegistry                    string
	SkillsAPIURL                   string
	SkillsAPISearchLimit           int
	MainAgentModel                 string
	DatabaseDriver                 string
	DatabaseURL                    string
	AccessToken                    string
	AuthSessionCookieName          string
	AuthCookieSameSite             string
	AuthCookieSecure               bool
	AuthSessionTTLHours            int
	DiscordBotToken                string
	TelegramBotToken               string
	ConnectorOAuthRedirectURI      string
	ConnectorGitHubClientID        string
	ConnectorGitHubClientSecret    string
	ConnectorGoogleClientID        string
	ConnectorGoogleClientSecret    string
	ConnectorLinkedInClientID      string
	ConnectorLinkedInClientSecret  string
	ConnectorTwitterClientID       string
	ConnectorTwitterClientSecret   string
	ConnectorInstagramClientID     string
	ConnectorInstagramClientSecret string
	ConnectorShopifyClientID       string
	ConnectorShopifyClientSecret   string
}

// Address 返回 http 服务监听地址。
func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Load 读取环境变量并构建配置。
func Load() Config {
	cacheDir := getenv("CACHE_FILE_DIR", "cache")
	debug := mustBool(getenv("DEBUG", "false"))
	logLevel := strings.TrimSpace(getenv("LOG_LEVEL", ""))
	if logLevel == "" {
		if debug {
			logLevel = "debug"
		} else {
			logLevel = "info"
		}
	}
	logFormat := strings.TrimSpace(getenv("LOG_FORMAT", ""))
	if logFormat == "" {
		if debug {
			logFormat = "text"
		} else {
			logFormat = "json"
		}
	}
	return Config{
		Host:                           getenv("HOST", "0.0.0.0"),
		Port:                           mustInt(getenv("PORT", "8010")),
		Debug:                          debug,
		ProjectName:                    getenv("PROJECT_NAME", "nexus"),
		LogLevel:                       logLevel,
		LogFormat:                      logFormat,
		LogPath:                        getenv("LOG_PATH", "~/.nexus/logs/logger.log"),
		LogStdout:                      mustBool(getenv("LOG_STDOUT", "true")),
		LogFileEnabled:                 mustBool(getenv("LOG_FILE_ENABLED", "true")),
		LogRotateDaily:                 mustBool(getenv("LOG_ROTATE_DAILY", "true")),
		LogMaxSizeMB:                   mustInt(getenv("LOG_MAX_SIZE_MB", "10")),
		LogMaxAgeDays:                  mustInt(getenv("LOG_MAX_AGE_DAYS", "7")),
		LogMaxBackups:                  mustInt(getenv("LOG_MAX_BACKUPS", "7")),
		LogCompress:                    mustBool(getenv("LOG_COMPRESS", "true")),
		APIPrefix:                      getenv("API_PREFIX", "/agent/v1"),
		WebSocketPath:                  getenv("WEBSOCKET_PATH", "/agent/v1/chat/ws"),
		DefaultAgentID:                 getenv("DEFAULT_AGENT_ID", "nexus"),
		WorkspacePath:                  getenv("WORKSPACE_PATH", ""),
		CacheFileDir:                   cacheDir,
		NpmRegistry:                    getenv("NPM_REGISTRY", ""),
		SkillsAPIURL:                   getenv("SKILLS_API_URL", "https://skills.sh"),
		SkillsAPISearchLimit:           mustInt(getenv("SKILLS_API_SEARCH_LIMIT", "20")),
		MainAgentModel:                 getenv("MAIN_AGENT_MODEL", getenv("DEFAULT_MODEL", "")),
		DatabaseDriver:                 getenv("DATABASE_DRIVER", "sqlite"),
		DatabaseURL:                    getenv("DATABASE_URL", "~/.nexus/data/nexus.db"),
		AccessToken:                    getenv("ACCESS_TOKEN", ""),
		AuthSessionCookieName:          getenv("AUTH_SESSION_COOKIE_NAME", "nexus_session"),
		AuthCookieSameSite:             getenv("AUTH_COOKIE_SAMESITE", "lax"),
		AuthCookieSecure:               mustBool(getenv("AUTH_COOKIE_SECURE", "false")),
		AuthSessionTTLHours:            mustInt(getenv("AUTH_SESSION_TTL_HOURS", "24")),
		DiscordBotToken:                getenv("DISCORD_BOT_TOKEN", ""),
		TelegramBotToken:               getenv("TELEGRAM_BOT_TOKEN", ""),
		ConnectorOAuthRedirectURI:      getenv("CONNECTOR_OAUTH_REDIRECT_URI", "http://localhost:3000/capability/connectors"),
		ConnectorGitHubClientID:        getenv("CONNECTOR_GITHUB_CLIENT_ID", ""),
		ConnectorGitHubClientSecret:    getenv("CONNECTOR_GITHUB_CLIENT_SECRET", ""),
		ConnectorGoogleClientID:        getenv("CONNECTOR_GOOGLE_CLIENT_ID", ""),
		ConnectorGoogleClientSecret:    getenv("CONNECTOR_GOOGLE_CLIENT_SECRET", ""),
		ConnectorLinkedInClientID:      getenv("CONNECTOR_LINKEDIN_CLIENT_ID", ""),
		ConnectorLinkedInClientSecret:  getenv("CONNECTOR_LINKEDIN_CLIENT_SECRET", ""),
		ConnectorTwitterClientID:       getenv("CONNECTOR_TWITTER_CLIENT_ID", ""),
		ConnectorTwitterClientSecret:   getenv("CONNECTOR_TWITTER_CLIENT_SECRET", ""),
		ConnectorInstagramClientID:     getenv("CONNECTOR_INSTAGRAM_CLIENT_ID", ""),
		ConnectorInstagramClientSecret: getenv("CONNECTOR_INSTAGRAM_CLIENT_SECRET", ""),
		ConnectorShopifyClientID:       getenv("CONNECTOR_SHOPIFY_CLIENT_ID", ""),
		ConnectorShopifyClientSecret:   getenv("CONNECTOR_SHOPIFY_CLIENT_SECRET", ""),
	}
}

func getenv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func mustInt(raw string) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 8010
	}
	return value
}

func mustBool(raw string) bool {
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return value
}
