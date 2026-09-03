package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// Config 承载 Go 服务运行时配置。
type Config struct {
	Host                             string
	Port                             int
	Debug                            bool
	ProjectName                      string
	LogLevel                         string
	LogFormat                        string
	LogPath                          string
	LogStdout                        bool
	LogNoColor                       bool
	LogFileEnabled                   bool
	LogRotateDaily                   bool
	LogMaxSizeMB                     int
	LogMaxAgeDays                    int
	LogMaxBackups                    int
	LogCompress                      bool
	MessageDebugStreamEvent          bool
	APIPrefix                        string
	WebSocketPath                    string
	DefaultAgentID                   string
	DefaultTimezone                  string
	WorkspacePath                    string
	CacheFileDir                     string
	WebDistDir                       string
	AppMode                          string
	DesktopSessionToken              string
	BrowserEnabled                   bool
	SkillsAPIURL                     string
	SkillsSourceURLs                 string
	SkillsDefaultSourcesEnabled      bool
	SkillsAPISearchLimit             int
	SkillsPrivateSourceAllowedHosts  []string
	DatabaseDriver                   string
	DatabaseURL                      string
	AuthSessionCookieName            string
	ControlURL                       string
	ControlServiceToken              string
	ControlServiceTokenFile          string
	ControlPrincipalPublicKey        string
	ControlPrincipalPublicKeyFile    string
	ControlPrincipalAudience         string
	ControlRequestTimeoutSeconds     int
	BaseSystemPrompt                 string
	MainAgentSystemPrompt            string
	MemoryMaintenance                MemoryMaintenanceConfig
	DiscordEnabled                   bool
	DiscordBotToken                  string
	TelegramEnabled                  bool
	TelegramBotToken                 string
	ConnectorOAuthRedirectURI        string
	ConnectorOAuthAllowedOrigins     []string
	AllowedWebSocketOrigins          []string
	ConnectorOAuthStateTTLSeconds    int
	GoalEnabled                      bool
	GoalAutoContinueEnabled          bool
	GoalMaxContinuationsPerRun       int
	AutomationRunTimeoutSeconds      int
	AutomationRecurringJitterSeconds int
	AutomationSchedulerLeaseSeconds  int
	AutomationMaxEnabledTasksPerUser int
	AutomationMisfirePolicy          string
	AutomationMisfireGraceSeconds    int
	RuntimeRoundIdleTimeoutSeconds   int
	RuntimeIdleSessionTTLSeconds     int
	RuntimeIdleSessionSweepSeconds   int
	RuntimeIsolationMode             string
	RuntimeLauncherPath              string
	ConnectorCredentialsKey          string
	ConnectorCredentialsLegacyKeys   []string
	ConnectorCredentialsHostKeyMode  string
	ConnectorGitHubClientID          string
	ConnectorGitHubClientSecret      string
	ConnectorGoogleClientID          string
	ConnectorGoogleClientSecret      string
	ConnectorLinkedInClientID        string
	ConnectorLinkedInClientSecret    string
	ConnectorTwitterClientID         string
	ConnectorTwitterClientSecret     string
	ConnectorInstagramClientID       string
	ConnectorInstagramClientSecret   string
	ConnectorShopifyClientID         string
	ConnectorShopifyClientSecret     string
}

// MemoryMaintenanceConfig 描述 Nexus 唤醒 nxs 记忆维护任务的宿主策略。
type MemoryMaintenanceConfig struct {
	MaxConcurrent int
	RunTimeout    time.Duration
	SweepInterval time.Duration
}

// Address 返回 http 服务监听地址。
func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Load 读取环境变量并构建配置。
func Load() Config {
	_ = LoadDotEnv()
	appRoot := appfs.AppDir()
	stateRoot := appfs.StateRoot()
	cacheDir := normalizeLegacyHostPath(
		getEnv("CACHE_FILE_DIR", filepath.Join(appRoot, "cache")),
		stateRoot,
		"cache",
		filepath.Join(appRoot, "cache"),
	)
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
	appMode := getEnv("NEXUS_APP_MODE", "")
	controlURL := strings.TrimSpace(getEnv("NEXUS_CONTROL_URL", ""))
	if controlURL == "" && !strings.EqualFold(strings.TrimSpace(appMode), "desktop") {
		controlURL = "http://127.0.0.1:8020"
	}
	controlServiceTokenFile := getEnv(
		"NEXUS_CONTROL_SERVICE_TOKEN_FILE",
		filepath.Join(appRoot, "control", "control-service.token"),
	)
	controlServiceToken := strings.TrimSpace(getEnv("NEXUS_CONTROL_SERVICE_TOKEN", ""))
	if controlServiceToken == "" {
		if data, err := os.ReadFile(controlServiceTokenFile); err == nil {
			controlServiceToken = strings.TrimSpace(string(data))
		}
	}
	browserEnabled := mustBool(getEnv("NEXUS_BROWSER_ENABLED", strconv.FormatBool(appMode == "desktop")))
	return Config{
		Host:        getEnv("HOST", "0.0.0.0"),
		Port:        parseIntEnv(getEnv("PORT", "8010"), 8010),
		Debug:       debug,
		ProjectName: getEnv("PROJECT_NAME", "nexus"),
		LogLevel:    logLevel,
		LogFormat:   logFormat,
		LogPath: normalizeLegacyHostPath(
			getEnv("LOG_PATH", filepath.Join(appRoot, "logs", "logger.log")),
			stateRoot,
			"logs",
			filepath.Join(appRoot, "logs"),
		),
		LogStdout:                   mustBool(getEnv("LOG_STDOUT", "true")),
		LogNoColor:                  mustBool(getEnv("LOG_NO_COLOR", "false")),
		LogFileEnabled:              mustBool(getEnv("LOG_FILE_ENABLED", "true")),
		LogRotateDaily:              mustBool(getEnv("LOG_ROTATE_DAILY", "true")),
		LogMaxSizeMB:                parseIntEnv(getEnv("LOG_MAX_SIZE_MB", "10"), 10),
		LogMaxAgeDays:               parseIntEnv(getEnv("LOG_MAX_AGE_DAYS", "7"), 7),
		LogMaxBackups:               parseIntEnv(getEnv("LOG_MAX_BACKUPS", "7"), 7),
		LogCompress:                 mustBool(getEnv("LOG_COMPRESS", "true")),
		MessageDebugStreamEvent:     mustBool(getEnv("MESSAGE_DEBUG_STREAM_EVENT", "false")),
		APIPrefix:                   getEnv("API_PREFIX", "/nexus/v1"),
		WebSocketPath:               getEnv("WEBSOCKET_PATH", "/nexus/v1/chat/ws"),
		DefaultAgentID:              getEnv("DEFAULT_AGENT_ID", "nexus"),
		DefaultTimezone:             getEnv("DEFAULT_TIMEZONE", "Asia/Shanghai"),
		WorkspacePath:               workspacePath,
		CacheFileDir:                cacheDir,
		WebDistDir:                  getEnv("WEB_DIST_DIR", ""),
		AppMode:                     appMode,
		DesktopSessionToken:         getEnv("NEXUS_DESKTOP_SESSION_TOKEN", ""),
		BrowserEnabled:              browserEnabled,
		SkillsAPIURL:                getEnv("SKILLS_API_URL", "https://skills.sh"),
		SkillsSourceURLs:            getEnv("SKILLS_SOURCE_URLS", ""),
		SkillsDefaultSourcesEnabled: mustBool(getEnv("SKILLS_DEFAULT_SOURCES_ENABLED", "true")),
		SkillsAPISearchLimit:        parseIntEnv(getEnv("SKILLS_API_SEARCH_LIMIT", "20"), 20),
		SkillsPrivateSourceAllowedHosts: mustStringList(
			getEnv("SKILLS_PRIVATE_SOURCE_ALLOWED_HOSTS", ""),
		),
		DatabaseDriver: getEnv("DATABASE_DRIVER", "sqlite"),
		DatabaseURL: normalizeLegacyDatabaseURL(
			getEnv("DATABASE_URL", filepath.Join(appRoot, "data", "nexus.db")),
			stateRoot,
			filepath.Join(appRoot, "data"),
		),
		AuthSessionCookieName: getEnv("AUTH_SESSION_COOKIE_NAME", "nexus_session"),
		ControlURL:              controlURL,
		ControlServiceToken:     controlServiceToken,
		ControlServiceTokenFile: controlServiceTokenFile,
		ControlPrincipalPublicKey: getEnv(
			"NEXUS_CONTROL_PRINCIPAL_PUBLIC_KEY",
			"",
		),
		ControlPrincipalPublicKeyFile: getEnv(
			"NEXUS_CONTROL_PRINCIPAL_PUBLIC_KEY_FILE",
			filepath.Join(appRoot, "control", "control-signing.pub"),
		),
		ControlPrincipalAudience:     getEnv("NEXUS_CONTROL_PRINCIPAL_AUDIENCE", "nexus-runtime"),
		ControlRequestTimeoutSeconds: parseIntEnv(getEnv("NEXUS_CONTROL_REQUEST_TIMEOUT_SECONDS", "5"), 5),
		BaseSystemPrompt:             getEnv("BASE_SYSTEM_PROMPT", ""),
		MainAgentSystemPrompt:        getEnv("MAIN_AGENT_SYSTEM_PROMPT", ""),
		MemoryMaintenance: MemoryMaintenanceConfig{
			MaxConcurrent: parseIntEnv(getEnv("MEMORY_MAINTENANCE_MAX_CONCURRENT", "2"), 2),
			RunTimeout:    time.Duration(parseIntEnv(getEnv("MEMORY_MAINTENANCE_RUN_TIMEOUT_SECONDS", "3600"), 3600)) * time.Second,
			SweepInterval: time.Duration(parseIntEnv(getEnv("MEMORY_MAINTENANCE_SWEEP_SECONDS", "600"), 600)) * time.Second,
		},
		DiscordEnabled:                   mustBool(getEnv("DISCORD_ENABLED", "true")),
		DiscordBotToken:                  getEnv("DISCORD_BOT_TOKEN", ""),
		TelegramEnabled:                  mustBool(getEnv("TELEGRAM_ENABLED", "true")),
		TelegramBotToken:                 getEnv("TELEGRAM_BOT_TOKEN", ""),
		ConnectorOAuthRedirectURI:        getEnv("CONNECTOR_OAUTH_REDIRECT_URI", "http://localhost:3000/capability/connectors/oauth/callback"),
		ConnectorOAuthAllowedOrigins:     mustStringList(getEnv("CONNECTOR_OAUTH_ALLOWED_ORIGINS", "http://localhost:3000")),
		AllowedWebSocketOrigins:          mustStringList(getEnv("ALLOWED_WEBSOCKET_ORIGINS", "")),
		ConnectorOAuthStateTTLSeconds:    parseIntEnv(getEnv("CONNECTOR_OAUTH_STATE_TTL_SECONDS", "600"), 600),
		GoalEnabled:                      mustBool(getEnv("NEXUS_GOAL_ENABLED", "true")),
		GoalAutoContinueEnabled:          mustBool(getEnv("NEXUS_GOAL_AUTO_CONTINUE_ENABLED", "true")),
		GoalMaxContinuationsPerRun:       parseIntEnv(getEnv("NEXUS_GOAL_MAX_CONTINUATIONS_PER_RUN", "20"), 20),
		AutomationRunTimeoutSeconds:      parseIntEnv(getEnv("AUTOMATION_RUN_TIMEOUT_SECONDS", "21600"), 21600),
		AutomationRecurringJitterSeconds: parseIntEnv(getEnv("AUTOMATION_RECURRING_JITTER_MAX_SECONDS", "900"), 900),
		AutomationSchedulerLeaseSeconds:  parseIntEnv(getEnv("AUTOMATION_SCHEDULER_LEASE_SECONDS", "30"), 30),
		AutomationMaxEnabledTasksPerUser: parseIntEnv(getEnv("AUTOMATION_MAX_ENABLED_TASKS_PER_USER", "100"), 100),
		AutomationMisfirePolicy:          getEnv("AUTOMATION_MISFIRE_POLICY", "run_once"),
		AutomationMisfireGraceSeconds:    parseIntEnv(getEnv("AUTOMATION_MISFIRE_GRACE_SECONDS", "60"), 60),
		RuntimeRoundIdleTimeoutSeconds:   parseIntEnv(getEnv("RUNTIME_ROUND_IDLE_TIMEOUT_SECONDS", "1200"), 1200),
		RuntimeIdleSessionTTLSeconds:     parseIntEnv(getEnv("RUNTIME_IDLE_SESSION_TTL_SECONDS", "600"), 600),
		RuntimeIdleSessionSweepSeconds:   parseIntEnv(getEnv("RUNTIME_IDLE_SESSION_SWEEP_SECONDS", "120"), 120),
		RuntimeIsolationMode:             getEnv("NEXUS_RUNTIME_ISOLATION_MODE", "off"),
		RuntimeLauncherPath:              getEnv("NEXUS_RUNTIME_LAUNCHER_PATH", "/usr/local/libexec/nexus-runtime-launcher"),
		ConnectorCredentialsKey:          getEnv("CONNECTOR_CREDENTIALS_KEY", ""),
		ConnectorCredentialsLegacyKeys:   mustStringList(getEnv("CONNECTOR_CREDENTIALS_LEGACY_KEYS", "")),
		ConnectorCredentialsHostKeyMode:  getEnv("CONNECTOR_CREDENTIALS_HOST_KEY_MODE", "explicit"),
		ConnectorGitHubClientID:          getEnv("CONNECTOR_GITHUB_CLIENT_ID", ""),
		ConnectorGitHubClientSecret:      getEnv("CONNECTOR_GITHUB_CLIENT_SECRET", ""),
		ConnectorGoogleClientID:          getEnv("CONNECTOR_GOOGLE_CLIENT_ID", ""),
		ConnectorGoogleClientSecret:      getEnv("CONNECTOR_GOOGLE_CLIENT_SECRET", ""),
		ConnectorLinkedInClientID:        getEnv("CONNECTOR_LINKEDIN_CLIENT_ID", ""),
		ConnectorLinkedInClientSecret:    getEnv("CONNECTOR_LINKEDIN_CLIENT_SECRET", ""),
		ConnectorTwitterClientID:         getEnv("CONNECTOR_TWITTER_CLIENT_ID", ""),
		ConnectorTwitterClientSecret:     getEnv("CONNECTOR_TWITTER_CLIENT_SECRET", ""),
		ConnectorInstagramClientID:       getEnv("CONNECTOR_INSTAGRAM_CLIENT_ID", ""),
		ConnectorInstagramClientSecret:   getEnv("CONNECTOR_INSTAGRAM_CLIENT_SECRET", ""),
		ConnectorShopifyClientID:         getEnv("CONNECTOR_SHOPIFY_CLIENT_ID", ""),
		ConnectorShopifyClientSecret:     getEnv("CONNECTOR_SHOPIFY_CLIENT_SECRET", ""),
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

// normalizeLegacyHostPath 把上一版直接挂在 .nexus 下的宿主目录映射到
// .nexus/app。只改动明确位于旧状态根中的路径，不碰用户自定义目录。
func normalizeLegacyHostPath(raw string, stateRoot string, legacyName string, targetRoot string) string {
	value := strings.TrimSpace(expandLeadingHome(raw))
	legacyRoot := filepath.Join(filepath.Clean(stateRoot), legacyName)
	if value != legacyRoot && !strings.HasPrefix(value, legacyRoot+string(os.PathSeparator)) {
		return raw
	}
	relative, err := filepath.Rel(legacyRoot, value)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return raw
	}
	if relative == "." {
		return filepath.Clean(targetRoot)
	}
	return filepath.Join(targetRoot, relative)
}

func normalizeLegacyDatabaseURL(raw string, stateRoot string, targetRoot string) string {
	value := strings.TrimSpace(raw)
	prefix := ""
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "sqlite:///"):
		prefix = value[:len("sqlite:///")]
		value = value[len("sqlite:///"):]
	case strings.HasPrefix(lower, "sqlite://"):
		prefix = value[:len("sqlite://")]
		value = value[len("sqlite://"):]
	}
	normalizedPath := normalizeLegacyHostPath(
		value,
		stateRoot,
		"data",
		targetRoot,
	)
	if normalizedPath == value {
		return raw
	}
	return prefix + normalizedPath
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
