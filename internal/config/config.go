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
	LogNoColor                     bool
	LogFileEnabled                 bool
	LogRotateDaily                 bool
	LogMaxSizeMB                   int
	LogMaxAgeDays                  int
	LogMaxBackups                  int
	LogCompress                    bool
	APIPrefix                      string
	WebSocketPath                  string
	DefaultAgentID                 string
	DefaultTimezone                string
	WorkspacePath                  string
	CacheFileDir                   string
	PnpmRegistry                   string
	SkillsAPIURL                   string
	SkillsAPISearchLimit           int
	DatabaseDriver                 string
	DatabaseURL                    string
	AccessToken                    string
	AuthSessionCookieName          string
	AuthCookieSameSite             string
	AuthCookieSecure               bool
	AuthSessionTTLHours            int
	BaseSystemPrompt               string
	MainAgentSystemPrompt          string
	DiscordEnabled                 bool
	DiscordBotToken                string
	TelegramEnabled                bool
	TelegramBotToken               string
	ConnectorOAuthRedirectURI      string
	ConnectorOAuthAllowedOrigins   []string
	ConnectorOAuthStateTTLSeconds  int
	ConnectorCredentialsKey        string
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
	_ = LoadDotEnv()
	cacheDir := getEnv("CACHE_FILE_DIR", "cache")
	debug := mustBool(getEnv("DEBUG", "false"))
	logLevel := strings.TrimSpace(getEnv("LOG_LEVEL", ""))
	if logLevel == "" {
		if debug {
			logLevel = "debug"
		} else {
			logLevel = "info"
		}
	}
	logFormat := strings.TrimSpace(getEnv("LOG_FORMAT", ""))
	if logFormat == "" {
		if debug {
			logFormat = "pretty"
		} else {
			logFormat = "json"
		}
	}
	return Config{
		Host:                           getEnv("HOST", "0.0.0.0"),
		Port:                           mustInt(getEnv("PORT", "8010")),
		Debug:                          debug,
		ProjectName:                    getEnv("PROJECT_NAME", "nexus"),
		LogLevel:                       logLevel,
		LogFormat:                      logFormat,
		LogPath:                        getEnv("LOG_PATH", "~/.nexus/logs/logger.log"),
		LogStdout:                      mustBool(getEnv("LOG_STDOUT", "true")),
		LogNoColor:                     mustBool(getEnv("LOG_NO_COLOR", "false")),
		LogFileEnabled:                 mustBool(getEnv("LOG_FILE_ENABLED", "true")),
		LogRotateDaily:                 mustBool(getEnv("LOG_ROTATE_DAILY", "true")),
		LogMaxSizeMB:                   mustInt(getEnv("LOG_MAX_SIZE_MB", "10")),
		LogMaxAgeDays:                  mustInt(getEnv("LOG_MAX_AGE_DAYS", "7")),
		LogMaxBackups:                  mustInt(getEnv("LOG_MAX_BACKUPS", "7")),
		LogCompress:                    mustBool(getEnv("LOG_COMPRESS", "true")),
		APIPrefix:                      getEnv("API_PREFIX", "/nexus/v1"),
		WebSocketPath:                  getEnv("WEBSOCKET_PATH", "/nexus/v1/chat/ws"),
		DefaultAgentID:                 getEnv("DEFAULT_AGENT_ID", "nexus"),
		DefaultTimezone:                getEnv("DEFAULT_TIMEZONE", "Asia/Shanghai"),
		WorkspacePath:                  getEnv("WORKSPACE_PATH", ""),
		CacheFileDir:                   cacheDir,
		PnpmRegistry:                   getEnv("PNPM_REGISTRY", ""),
		SkillsAPIURL:                   getEnv("SKILLS_API_URL", "https://skills.sh"),
		SkillsAPISearchLimit:           mustInt(getEnv("SKILLS_API_SEARCH_LIMIT", "20")),
		DatabaseDriver:                 getEnv("DATABASE_DRIVER", "sqlite"),
		DatabaseURL:                    getEnv("DATABASE_URL", "~/.nexus/data/nexus.db"),
		AccessToken:                    getEnv("ACCESS_TOKEN", ""),
		AuthSessionCookieName:          getEnv("AUTH_SESSION_COOKIE_NAME", "nexus_session"),
		AuthCookieSameSite:             getEnv("AUTH_COOKIE_SAMESITE", "lax"),
		AuthCookieSecure:               mustBool(getEnv("AUTH_COOKIE_SECURE", "false")),
		AuthSessionTTLHours:            mustInt(getEnv("AUTH_SESSION_TTL_HOURS", "24")),
		BaseSystemPrompt:               getEnv("BASE_SYSTEM_PROMPT", ""),
		MainAgentSystemPrompt:          getEnv("MAIN_AGENT_SYSTEM_PROMPT", ""),
		DiscordEnabled:                 mustBool(getEnv("DISCORD_ENABLED", "true")),
		DiscordBotToken:                getEnv("DISCORD_BOT_TOKEN", ""),
		TelegramEnabled:                mustBool(getEnv("TELEGRAM_ENABLED", "true")),
		TelegramBotToken:               getEnv("TELEGRAM_BOT_TOKEN", ""),
		ConnectorOAuthRedirectURI:      getEnv("CONNECTOR_OAUTH_REDIRECT_URI", "http://localhost:3000/capability/connectors/oauth/callback"),
		ConnectorOAuthAllowedOrigins:   mustStringList(getEnv("CONNECTOR_OAUTH_ALLOWED_ORIGINS", "http://localhost:3000")),
		ConnectorOAuthStateTTLSeconds:  mustInt(getEnv("CONNECTOR_OAUTH_STATE_TTL_SECONDS", "600")),
		ConnectorCredentialsKey:        getEnv("CONNECTOR_CREDENTIALS_KEY", ""),
		ConnectorGitHubClientID:        getEnv("CONNECTOR_GITHUB_CLIENT_ID", ""),
		ConnectorGitHubClientSecret:    getEnv("CONNECTOR_GITHUB_CLIENT_SECRET", ""),
		ConnectorGoogleClientID:        getEnv("CONNECTOR_GOOGLE_CLIENT_ID", ""),
		ConnectorGoogleClientSecret:    getEnv("CONNECTOR_GOOGLE_CLIENT_SECRET", ""),
		ConnectorLinkedInClientID:      getEnv("CONNECTOR_LINKEDIN_CLIENT_ID", ""),
		ConnectorLinkedInClientSecret:  getEnv("CONNECTOR_LINKEDIN_CLIENT_SECRET", ""),
		ConnectorTwitterClientID:       getEnv("CONNECTOR_TWITTER_CLIENT_ID", ""),
		ConnectorTwitterClientSecret:   getEnv("CONNECTOR_TWITTER_CLIENT_SECRET", ""),
		ConnectorInstagramClientID:     getEnv("CONNECTOR_INSTAGRAM_CLIENT_ID", ""),
		ConnectorInstagramClientSecret: getEnv("CONNECTOR_INSTAGRAM_CLIENT_SECRET", ""),
		ConnectorShopifyClientID:       getEnv("CONNECTOR_SHOPIFY_CLIENT_ID", ""),
		ConnectorShopifyClientSecret:   getEnv("CONNECTOR_SHOPIFY_CLIENT_SECRET", ""),
	}
}

func getEnv(key string, fallback string) string {
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

func mustStringList(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}
