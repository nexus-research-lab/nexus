package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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
	MessageDebugStreamEvent        bool
	APIPrefix                      string
	WebSocketPath                  string
	DefaultAgentID                 string
	DefaultTimezone                string
	WorkspacePath                  string
	CacheFileDir                   string
	WebDistDir                     string
	AppMode                        string
	DesktopSessionToken            string
	SkillsAPIURL                   string
	SkillsSourceURLs               string
	SkillsDefaultSourcesEnabled    bool
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
	MemoryEnabled                  bool
	MemoryAutoRecall               bool
	MemoryAutoExtract              bool
	MemoryMaxResults               int
	MemoryScoreThreshold           float64
	DiscordEnabled                 bool
	DiscordBotToken                string
	TelegramEnabled                bool
	TelegramBotToken               string
	ConnectorOAuthRedirectURI      string
	ConnectorOAuthAllowedOrigins   []string
	AllowedWebSocketOrigins        []string
	ConnectorOAuthStateTTLSeconds  int
	GoalEnabled                    bool
	GoalAutoContinueEnabled        bool
	GoalMaxContinuationsPerRun     int
	AutomationRunTimeoutSeconds    int
	RuntimeRoundIdleTimeoutSeconds int
	RuntimeIdleSessionTTLSeconds   int
	RuntimeIdleSessionSweepSeconds int
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
	workspacePath := configuredWorkspacePath(getEnv("WORKSPACE_PATH", ""))
	return Config{
		Host:                           getEnv("HOST", "0.0.0.0"),
		Port:                           parseIntEnv(getEnv("PORT", "8010"), 8010),
		Debug:                          debug,
		ProjectName:                    getEnv("PROJECT_NAME", "nexus"),
		LogLevel:                       logLevel,
		LogFormat:                      logFormat,
		LogPath:                        getEnv("LOG_PATH", "~/.nexus/logs/logger.log"),
		LogStdout:                      mustBool(getEnv("LOG_STDOUT", "true")),
		LogNoColor:                     mustBool(getEnv("LOG_NO_COLOR", "false")),
		LogFileEnabled:                 mustBool(getEnv("LOG_FILE_ENABLED", "true")),
		LogRotateDaily:                 mustBool(getEnv("LOG_ROTATE_DAILY", "true")),
		LogMaxSizeMB:                   parseIntEnv(getEnv("LOG_MAX_SIZE_MB", "10"), 10),
		LogMaxAgeDays:                  parseIntEnv(getEnv("LOG_MAX_AGE_DAYS", "7"), 7),
		LogMaxBackups:                  parseIntEnv(getEnv("LOG_MAX_BACKUPS", "7"), 7),
		LogCompress:                    mustBool(getEnv("LOG_COMPRESS", "true")),
		MessageDebugStreamEvent:        mustBool(getEnv("MESSAGE_DEBUG_STREAM_EVENT", "false")),
		APIPrefix:                      getEnv("API_PREFIX", "/nexus/v1"),
		WebSocketPath:                  getEnv("WEBSOCKET_PATH", "/nexus/v1/chat/ws"),
		DefaultAgentID:                 getEnv("DEFAULT_AGENT_ID", "nexus"),
		DefaultTimezone:                getEnv("DEFAULT_TIMEZONE", "Asia/Shanghai"),
		WorkspacePath:                  workspacePath,
		CacheFileDir:                   cacheDir,
		WebDistDir:                     getEnv("WEB_DIST_DIR", ""),
		AppMode:                        getEnv("NEXUS_APP_MODE", ""),
		DesktopSessionToken:            getEnv("NEXUS_DESKTOP_SESSION_TOKEN", ""),
		SkillsAPIURL:                   getEnv("SKILLS_API_URL", "https://skills.sh"),
		SkillsSourceURLs:               getEnv("SKILLS_SOURCE_URLS", ""),
		SkillsDefaultSourcesEnabled:    mustBool(getEnv("SKILLS_DEFAULT_SOURCES_ENABLED", "true")),
		SkillsAPISearchLimit:           parseIntEnv(getEnv("SKILLS_API_SEARCH_LIMIT", "20"), 20),
		DatabaseDriver:                 getEnv("DATABASE_DRIVER", "sqlite"),
		DatabaseURL:                    getEnv("DATABASE_URL", "~/.nexus/data/nexus.db"),
		AccessToken:                    getEnv("ACCESS_TOKEN", ""),
		AuthSessionCookieName:          getEnv("AUTH_SESSION_COOKIE_NAME", "nexus_session"),
		AuthCookieSameSite:             getEnv("AUTH_COOKIE_SAMESITE", "lax"),
		AuthCookieSecure:               mustBool(getEnv("AUTH_COOKIE_SECURE", "false")),
		AuthSessionTTLHours:            parseIntEnv(getEnv("AUTH_SESSION_TTL_HOURS", "24"), 24),
		BaseSystemPrompt:               getEnv("BASE_SYSTEM_PROMPT", ""),
		MainAgentSystemPrompt:          getEnv("MAIN_AGENT_SYSTEM_PROMPT", ""),
		MemoryEnabled:                  mustBool(getEnv("MEMORY_ENABLED", "true")),
		MemoryAutoRecall:               mustBool(getEnv("MEMORY_AUTO_RECALL", "true")),
		MemoryAutoExtract:              mustBool(getEnv("MEMORY_AUTO_EXTRACT", "true")),
		MemoryMaxResults:               parseIntEnv(getEnv("MEMORY_MAX_RESULTS", "5"), 5),
		MemoryScoreThreshold:           mustFloat(getEnv("MEMORY_SCORE_THRESHOLD", "0.08")),
		DiscordEnabled:                 mustBool(getEnv("DISCORD_ENABLED", "true")),
		DiscordBotToken:                getEnv("DISCORD_BOT_TOKEN", ""),
		TelegramEnabled:                mustBool(getEnv("TELEGRAM_ENABLED", "true")),
		TelegramBotToken:               getEnv("TELEGRAM_BOT_TOKEN", ""),
		ConnectorOAuthRedirectURI:      getEnv("CONNECTOR_OAUTH_REDIRECT_URI", "http://localhost:3000/capability/connectors/oauth/callback"),
		ConnectorOAuthAllowedOrigins:   mustStringList(getEnv("CONNECTOR_OAUTH_ALLOWED_ORIGINS", "http://localhost:3000")),
		AllowedWebSocketOrigins:        mustStringList(getEnv("ALLOWED_WEBSOCKET_ORIGINS", "")),
		ConnectorOAuthStateTTLSeconds:  parseIntEnv(getEnv("CONNECTOR_OAUTH_STATE_TTL_SECONDS", "600"), 600),
		GoalEnabled:                    mustBool(getEnv("NEXUS_GOAL_ENABLED", "true")),
		GoalAutoContinueEnabled:        mustBool(getEnv("NEXUS_GOAL_AUTO_CONTINUE_ENABLED", "true")),
		GoalMaxContinuationsPerRun:     parseIntEnv(getEnv("NEXUS_GOAL_MAX_CONTINUATIONS_PER_RUN", "20"), 20),
		AutomationRunTimeoutSeconds:    parseIntEnv(getEnv("AUTOMATION_RUN_TIMEOUT_SECONDS", "21600"), 21600),
		RuntimeRoundIdleTimeoutSeconds: parseIntEnv(getEnv("RUNTIME_ROUND_IDLE_TIMEOUT_SECONDS", "1200"), 1200),
		RuntimeIdleSessionTTLSeconds:   parseIntEnv(getEnv("RUNTIME_IDLE_SESSION_TTL_SECONDS", "600"), 600),
		RuntimeIdleSessionSweepSeconds: parseIntEnv(getEnv("RUNTIME_IDLE_SESSION_SWEEP_SECONDS", "120"), 120),
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

// RuntimeRoundIdleTimeout 返回单轮 runtime 流无事件保护时长，<=0 表示使用 runtime 默认值。
func (c Config) RuntimeRoundIdleTimeout() time.Duration {
	if c.RuntimeRoundIdleTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(c.RuntimeRoundIdleTimeoutSeconds) * time.Second
}

// RuntimeIdleSessionTTL 返回无运行 round 的 SDK client 保留时长，<=0 表示关闭回收。
func (c Config) RuntimeIdleSessionTTL() time.Duration {
	if c.RuntimeIdleSessionTTLSeconds <= 0 {
		return 0
	}
	return time.Duration(c.RuntimeIdleSessionTTLSeconds) * time.Second
}

// RuntimeIdleSessionSweepInterval 返回 runtime 空闲 session 扫描间隔，<=0 表示关闭回收。
func (c Config) RuntimeIdleSessionSweepInterval() time.Duration {
	if c.RuntimeIdleSessionSweepSeconds <= 0 {
		return 0
	}
	return time.Duration(c.RuntimeIdleSessionSweepSeconds) * time.Second
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func parseIntEnv(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
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

func mustFloat(raw string) float64 {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
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
