// INPUT: 进程启动 Config。
// OUTPUT: 显式白名单内的非敏感启动设置与 credential configured 状态。
// POS: host 配置域的安全投影；禁止把完整 Config 交给启发式脱敏。
package configuration

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func hostStartupConfigurationSnapshot(cfg config.Config) map[string]any {
	return map[string]any{
		"server": map[string]any{
			"host": cfg.Host, "port": cfg.Port, "debug": cfg.Debug,
			"project_name": cfg.ProjectName, "api_prefix": cfg.APIPrefix,
			"websocket_path": cfg.WebSocketPath, "app_mode": cfg.AppMode,
		},
		"logging": map[string]any{
			"level": cfg.LogLevel, "format": cfg.LogFormat, "path": cfg.LogPath,
			"stdout": cfg.LogStdout, "no_color": cfg.LogNoColor,
			"file_enabled": cfg.LogFileEnabled, "rotate_daily": cfg.LogRotateDaily,
			"max_size_mb": cfg.LogMaxSizeMB, "max_age_days": cfg.LogMaxAgeDays,
			"max_backups": cfg.LogMaxBackups, "compress": cfg.LogCompress,
			"message_debug_stream_event": cfg.MessageDebugStreamEvent,
		},
		"paths": map[string]any{
			"workspace_path": cfg.WorkspacePath, "cache_file_dir": cfg.CacheFileDir,
			"web_dist_dir": cfg.WebDistDir,
		},
		"defaults": map[string]any{
			"agent_id": cfg.DefaultAgentID, "timezone": cfg.DefaultTimezone,
		},
		"skills": map[string]any{
			"api_url": cfg.SkillsAPIURL, "source_urls": cfg.SkillsSourceURLs,
			"default_sources_enabled": cfg.SkillsDefaultSourcesEnabled,
			"api_search_limit":        cfg.SkillsAPISearchLimit,
		},
		"database": map[string]any{
			"driver": cfg.DatabaseDriver,
			"url":    secretConfigured(cfg.DatabaseURL),
		},
		"auth": map[string]any{
			"session_cookie_name": cfg.AuthSessionCookieName,
			"desktop_session_token": secretConfigured(cfg.DesktopSessionToken),
		},
		"memory_maintenance": map[string]any{
			"max_concurrent": cfg.MemoryMaintenance.MaxConcurrent,
			"run_timeout":    cfg.MemoryMaintenance.RunTimeout.String(),
			"sweep_interval": cfg.MemoryMaintenance.SweepInterval.String(),
		},
		"channels": map[string]any{
			"discord_enabled":    cfg.DiscordEnabled,
			"discord_bot_token":  secretConfigured(cfg.DiscordBotToken),
			"telegram_enabled":   cfg.TelegramEnabled,
			"telegram_bot_token": secretConfigured(cfg.TelegramBotToken),
		},
		"connector_oauth": map[string]any{
			"redirect_uri":      cfg.ConnectorOAuthRedirectURI,
			"allowed_origins":   cfg.ConnectorOAuthAllowedOrigins,
			"state_ttl_seconds": cfg.ConnectorOAuthStateTTLSeconds,
			"credentials_key":   secretConfigured(cfg.ConnectorCredentialsKey),
			"clients": map[string]any{
				"github":    connectorClientStatus(cfg.ConnectorGitHubClientID, cfg.ConnectorGitHubClientSecret),
				"google":    connectorClientStatus(cfg.ConnectorGoogleClientID, cfg.ConnectorGoogleClientSecret),
				"linkedin":  connectorClientStatus(cfg.ConnectorLinkedInClientID, cfg.ConnectorLinkedInClientSecret),
				"twitter":   connectorClientStatus(cfg.ConnectorTwitterClientID, cfg.ConnectorTwitterClientSecret),
				"instagram": connectorClientStatus(cfg.ConnectorInstagramClientID, cfg.ConnectorInstagramClientSecret),
				"shopify":   connectorClientStatus(cfg.ConnectorShopifyClientID, cfg.ConnectorShopifyClientSecret),
			},
		},
		"allowed_websocket_origins": cfg.AllowedWebSocketOrigins,
		"goal": map[string]any{
			"enabled": cfg.GoalEnabled, "auto_continue_enabled": cfg.GoalAutoContinueEnabled,
			"max_continuations_per_run": cfg.GoalMaxContinuationsPerRun,
		},
		"automation": map[string]any{
			"run_timeout_seconds":        cfg.AutomationRunTimeoutSeconds,
			"recurring_jitter_seconds":   cfg.AutomationRecurringJitterSeconds,
			"scheduler_lease_seconds":    cfg.AutomationSchedulerLeaseSeconds,
			"max_enabled_tasks_per_user": cfg.AutomationMaxEnabledTasksPerUser,
			"misfire_policy":             cfg.AutomationMisfirePolicy,
			"misfire_grace_seconds":      cfg.AutomationMisfireGraceSeconds,
		},
		"runtime_lifecycle": map[string]any{
			"round_idle_timeout_seconds": cfg.RuntimeRoundIdleTimeoutSeconds,
			"idle_session_ttl_seconds":   cfg.RuntimeIdleSessionTTLSeconds,
			"idle_session_sweep_seconds": cfg.RuntimeIdleSessionSweepSeconds,
		},
		"prompt_overrides": map[string]any{
			"base":       secretConfigured(cfg.BaseSystemPrompt),
			"main_agent": secretConfigured(cfg.MainAgentSystemPrompt),
		},
	}
}

func connectorClientStatus(clientID, clientSecret string) map[string]bool {
	return map[string]bool{
		"client_id_configured":     strings.TrimSpace(clientID) != "",
		"client_secret_configured": strings.TrimSpace(clientSecret) != "",
	}
}

func secretConfigured(value string) map[string]any {
	return map[string]any{
		"configured": strings.TrimSpace(value) != "",
		"redacted":   true,
	}
}
