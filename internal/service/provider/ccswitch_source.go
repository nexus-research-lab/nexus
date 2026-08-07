// INPUT: CC Switch 的 SQLite 单一真相源、settings.json 与 Codex TOML 片段。
// OUTPUT: 已脱敏、可判定兼容性的 Provider 候选集合。
// POS: CC Switch 只读适配层；绝不写入外部数据库或返回原始凭据。
package provider

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	ccSwitchDatabaseName = "cc-switch.db"
	ccSwitchSettingsName = "settings.json"
)

var ccSwitchLongContextSuffix = regexp.MustCompile(`(?i)\s*\[1m\]\s*$`)

type ccSwitchProviderRow struct {
	id             string
	appType        string
	name           string
	settingsConfig string
	meta           string
	isCurrent      bool
}

type ccSwitchDeviceSettings struct {
	CurrentProviderClaude string `json:"currentProviderClaude"`
	CurrentProviderCodex  string `json:"currentProviderCodex"`
}

type ccSwitchCodexConfig struct {
	Model                   string                               `toml:"model"`
	ModelProvider           string                               `toml:"model_provider"`
	ExperimentalBearerToken string                               `toml:"experimental_bearer_token"`
	ModelProviders          map[string]ccSwitchCodexProviderTOML `toml:"model_providers"`
}

type ccSwitchCodexProviderTOML struct {
	BaseURL                 string `toml:"base_url"`
	WireAPI                 string `toml:"wire_api"`
	ExperimentalBearerToken string `toml:"experimental_bearer_token"`
}

func (s *Service) readCCSwitchSource(ctx context.Context, configuredPath string) (ccSwitchSource, error) {
	configDir, databasePath, err := resolveCCSwitchPaths(configuredPath)
	if err != nil {
		return ccSwitchSource{}, err
	}
	if _, err = os.Stat(databasePath); err != nil {
		return ccSwitchSource{configDir: configDir, databasePath: databasePath}, err
	}
	database, err := openReadOnlySQLite(databasePath)
	if err != nil {
		return ccSwitchSource{}, fmt.Errorf("打开 CC Switch 数据库失败: %w", err)
	}
	defer database.Close()

	rows, version, err := readCCSwitchProviderRows(ctx, database)
	if err != nil {
		return ccSwitchSource{}, err
	}
	settings := readCCSwitchDeviceSettings(filepath.Join(configDir, ccSwitchSettingsName))
	applyCCSwitchCurrentSelection(rows, settings)
	candidates := make([]ccSwitchCandidate, 0, len(rows))
	for _, row := range rows {
		candidate := buildCCSwitchCandidate(row)
		if candidate.preview.SourceKey == "" || isEmptyCCSwitchBuiltIn(row, candidate) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(left int, right int) bool {
		if candidates[left].preview.Current != candidates[right].preview.Current {
			return candidates[left].preview.Current
		}
		if candidates[left].preview.AppType != candidates[right].preview.AppType {
			return candidates[left].preview.AppType < candidates[right].preview.AppType
		}
		return strings.ToLower(candidates[left].preview.Name) < strings.ToLower(candidates[right].preview.Name)
	})
	return ccSwitchSource{
		configDir:     configDir,
		databasePath:  databasePath,
		schemaVersion: version,
		candidates:    candidates,
	}, nil
}

func isEmptyCCSwitchBuiltIn(row ccSwitchProviderRow, candidate ccSwitchCandidate) bool {
	if strings.TrimSpace(candidate.authToken) != "" {
		return false
	}
	providerID := strings.ToLower(strings.TrimSpace(row.id))
	return providerID == "default" || strings.HasSuffix(providerID, "-official")
}

func resolveCCSwitchPaths(configuredPath string) (string, string, error) {
	value := strings.TrimSpace(configuredPath)
	if value == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("读取用户目录失败: %w", err)
		}
		value = filepath.Join(home, ".cc-switch")
	} else if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("读取用户目录失败: %w", err)
		}
		value = filepath.Join(home, strings.TrimLeft(value[1:], `/\`))
	}
	absolute, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return "", "", fmt.Errorf("解析 CC Switch 路径失败: %w", err)
	}
	if strings.EqualFold(filepath.Ext(absolute), ".db") {
		return filepath.Dir(absolute), absolute, nil
	}
	return absolute, filepath.Join(absolute, ccSwitchDatabaseName), nil
}

func openReadOnlySQLite(path string) (*sql.DB, error) {
	databaseURL := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := databaseURL.Query()
	query.Set("mode", "ro")
	databaseURL.RawQuery = query.Encode()
	database, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	if err = database.Ping(); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

func readCCSwitchProviderRows(ctx context.Context, database *sql.DB) ([]ccSwitchProviderRow, int, error) {
	columns, err := ccSwitchProviderColumns(ctx, database)
	if err != nil {
		return nil, 0, err
	}
	for _, required := range []string{"id", "app_type", "name", "settings_config"} {
		if !columns[required] {
			return nil, 0, fmt.Errorf("CC Switch 数据库缺少 providers.%s", required)
		}
	}
	metaExpression := "'{}'"
	if columns["meta"] {
		metaExpression = "COALESCE(meta, '{}')"
	}
	currentExpression := "0"
	if columns["is_current"] {
		currentExpression = "COALESCE(is_current, 0)"
	}
	query := fmt.Sprintf(`
SELECT id, app_type, name, settings_config, %s, %s
FROM providers
WHERE app_type IN ('claude', 'codex')`, metaExpression, currentExpression)
	result, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("读取 CC Switch providers 失败: %w", err)
	}
	defer result.Close()
	items := make([]ccSwitchProviderRow, 0)
	for result.Next() {
		var item ccSwitchProviderRow
		if err = result.Scan(
			&item.id,
			&item.appType,
			&item.name,
			&item.settingsConfig,
			&item.meta,
			&item.isCurrent,
		); err != nil {
			return nil, 0, fmt.Errorf("解析 CC Switch provider 失败: %w", err)
		}
		items = append(items, item)
	}
	if err = result.Err(); err != nil {
		return nil, 0, err
	}
	var version int
	_ = database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
	return items, version, nil
}

func ccSwitchProviderColumns(ctx context.Context, database *sql.DB) (map[string]bool, error) {
	rows, err := database.QueryContext(ctx, "PRAGMA table_info(providers)")
	if err != nil {
		return nil, fmt.Errorf("读取 CC Switch 数据库结构失败: %w", err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var sequence int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err = rows.Scan(&sequence, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[strings.ToLower(strings.TrimSpace(name))] = true
	}
	return columns, rows.Err()
}

func readCCSwitchDeviceSettings(path string) ccSwitchDeviceSettings {
	content, err := os.ReadFile(path)
	if err != nil {
		return ccSwitchDeviceSettings{}
	}
	var settings ccSwitchDeviceSettings
	if json.Unmarshal(content, &settings) == nil {
		return settings
	}
	return ccSwitchDeviceSettings{}
}

func applyCCSwitchCurrentSelection(rows []ccSwitchProviderRow, settings ccSwitchDeviceSettings) {
	configured := map[string]string{
		ccSwitchAppClaude: strings.TrimSpace(settings.CurrentProviderClaude),
		ccSwitchAppCodex:  strings.TrimSpace(settings.CurrentProviderCodex),
	}
	exists := map[string]map[string]bool{}
	for _, row := range rows {
		if exists[row.appType] == nil {
			exists[row.appType] = map[string]bool{}
		}
		exists[row.appType][row.id] = true
	}
	for index := range rows {
		selected := configured[rows[index].appType]
		if selected != "" && exists[rows[index].appType][selected] {
			rows[index].isCurrent = rows[index].id == selected
		}
	}
}

func buildCCSwitchCandidate(row ccSwitchProviderRow) ccSwitchCandidate {
	settings := decodeJSONObject(row.settingsConfig)
	meta := decodeJSONObject(row.meta)
	base := CCSwitchProviderPreview{
		SourceKey: row.appType + ":" + strings.TrimSpace(row.id),
		AppType:   strings.TrimSpace(row.appType),
		Name:      firstNonEmpty(row.name, row.id),
		Provider:  ccSwitchProviderKey(row.appType, row.id, row.name),
		Current:   row.isCurrent,
		Models:    []CCSwitchModelPreview{},
	}
	var candidate ccSwitchCandidate
	switch base.AppType {
	case ccSwitchAppClaude:
		candidate = buildCCSwitchClaudeCandidate(base, settings, meta)
	case ccSwitchAppCodex:
		candidate = buildCCSwitchCodexCandidate(base, settings, meta)
	default:
		return ccSwitchCandidate{}
	}
	finalizeCCSwitchCandidate(&candidate, meta)
	return candidate
}

func buildCCSwitchClaudeCandidate(
	preview CCSwitchProviderPreview,
	settings map[string]any,
	meta map[string]any,
) ccSwitchCandidate {
	environment := jsonMap(settings["env"])
	preview.BaseURL = firstNonEmpty(jsonString(environment["ANTHROPIC_BASE_URL"]), "https://api.anthropic.com")
	preview.APIFormat = ccSwitchAPIFormat(jsonString(meta["apiFormat"]), "anthropic")
	preview.Models, preview.DefaultModel = ccSwitchClaudeModels(environment)
	keyField := strings.ToUpper(jsonString(meta["apiKeyField"]))
	if keyField == "ANTHROPIC_API_KEY" {
		return ccSwitchCandidate{
			preview:   preview,
			authToken: jsonString(environment["ANTHROPIC_API_KEY"]),
		}
	}
	return ccSwitchCandidate{
		preview: preview,
		authToken: firstNonEmpty(
			jsonString(environment["ANTHROPIC_AUTH_TOKEN"]),
			jsonString(environment["ANTHROPIC_API_KEY"]),
		),
	}
}

func buildCCSwitchCodexCandidate(
	preview CCSwitchProviderPreview,
	settings map[string]any,
	meta map[string]any,
) ccSwitchCandidate {
	auth := jsonMap(settings["auth"])
	configText := jsonString(settings["config"])
	var config ccSwitchCodexConfig
	_ = toml.Unmarshal([]byte(configText), &config)
	providerConfig := config.ModelProviders[config.ModelProvider]
	if config.ModelProvider == "" && len(config.ModelProviders) == 1 {
		for _, item := range config.ModelProviders {
			providerConfig = item
		}
	}
	preview.BaseURL = firstNonEmpty(providerConfig.BaseURL, "https://api.openai.com/v1")
	preview.APIFormat = ccSwitchAPIFormat(
		jsonString(meta["apiFormat"]),
		firstNonEmpty(providerConfig.WireAPI, "responses"),
	)
	preview.Models, preview.DefaultModel = ccSwitchCodexModels(settings, config.Model, meta)
	return ccSwitchCandidate{
		preview: preview,
		authToken: firstNonEmpty(
			jsonString(auth["OPENAI_API_KEY"]),
			providerConfig.ExperimentalBearerToken,
			config.ExperimentalBearerToken,
		),
	}
}

func finalizeCCSwitchCandidate(candidate *ccSwitchCandidate, meta map[string]any) {
	preview := &candidate.preview
	preview.RuntimeSupport = ccSwitchRuntimeSupport(preview.APIFormat)
	preview.Status = ccSwitchStatusReady
	preview.CanSync = true
	if reason := ccSwitchUnsupportedReason(*candidate, meta); reason != "" {
		preview.Status = ccSwitchStatusUnsupported
		preview.Reason = reason
		preview.CanSync = false
		return
	}
	missing := make([]string, 0, 3)
	if strings.TrimSpace(candidate.authToken) == "" || strings.EqualFold(strings.TrimSpace(candidate.authToken), "PROXY_MANAGED") {
		missing = append(missing, "API Key")
	}
	if strings.TrimSpace(preview.BaseURL) == "" {
		missing = append(missing, "Base URL")
	}
	if len(preview.Models) == 0 {
		missing = append(missing, "模型")
	}
	if len(missing) > 0 {
		preview.Status = ccSwitchStatusIncomplete
		preview.Reason = "缺少" + strings.Join(missing, "、")
		preview.CanSync = false
	}
}

func ccSwitchUnsupportedReason(candidate ccSwitchCandidate, meta map[string]any) string {
	providerType := strings.ToLower(jsonString(meta["providerType"]))
	switch providerType {
	case "codex_oauth", "github_copilot", "xai_oauth":
		return "托管账号暂不能迁移"
	}
	authBinding := jsonMap(meta["authBinding"])
	if strings.EqualFold(jsonString(authBinding["source"]), "managed_account") {
		return "托管账号暂不能迁移"
	}
	if jsonBool(meta["isFullUrl"]) {
		return "完整请求地址暂不支持"
	}
	if jsonValueHasContent(meta["localProxyRequestOverrides"]) {
		return "依赖 CC Switch 本地代理改写"
	}
	if len(ccSwitchRuntimeSupport(candidate.preview.APIFormat)) == 0 {
		return "接口格式暂不支持"
	}
	if candidate.preview.AppType == ccSwitchAppClaude &&
		strings.EqualFold(jsonString(meta["apiKeyField"]), "ANTHROPIC_API_KEY") &&
		!isOfficialAnthropicURL(candidate.preview.BaseURL) {
		return "第三方 ANTHROPIC_API_KEY 认证暂不支持"
	}
	return ""
}

func ccSwitchClaudeModels(environment map[string]any) ([]CCSwitchModelPreview, string) {
	keys := []string{
		"ANTHROPIC_MODEL",
		"ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL",
		"ANTHROPIC_DEFAULT_FABLE_MODEL",
		"CLAUDE_CODE_SUBAGENT_MODEL",
	}
	models := make([]CCSwitchModelPreview, 0, len(keys))
	seen := map[string]bool{}
	defaultModel := ""
	for _, key := range keys {
		raw := jsonString(environment[key])
		modelID, contextWindow := normalizeCCSwitchModelID(raw)
		if modelID == "" {
			continue
		}
		if defaultModel == "" {
			defaultModel = modelID
		}
		if seen[modelID] {
			continue
		}
		seen[modelID] = true
		models = append(models, CCSwitchModelPreview{
			ModelID:       modelID,
			DisplayName:   modelDisplayName(modelID, jsonString(environment[key+"_NAME"])),
			ContextWindow: contextWindow,
			Capabilities:  []string{"tools"},
		})
	}
	return models, defaultModel
}

func ccSwitchCodexModels(
	settings map[string]any,
	configuredModel string,
	meta map[string]any,
) ([]CCSwitchModelPreview, string) {
	catalog := jsonMap(settings["modelCatalog"])
	items := jsonSlice(catalog["models"])
	models := make([]CCSwitchModelPreview, 0, len(items)+1)
	seen := map[string]bool{}
	add := func(modelID string, displayName string, contextWindow *int, modalities []string) {
		modelID = normalizeModelID(modelID)
		if modelID == "" || seen[modelID] {
			return
		}
		seen[modelID] = true
		capabilities := []string{"tools"}
		if stringSliceContainsFold(modalities, "image") {
			capabilities = append(capabilities, "vision")
		}
		thinking := jsonMap(meta["codexChatReasoning"])
		if jsonBool(thinking["supportsThinking"]) {
			capabilities = append(capabilities, "reasoning")
		}
		models = append(models, CCSwitchModelPreview{
			ModelID:       modelID,
			DisplayName:   modelDisplayName(modelID, displayName),
			ContextWindow: contextWindow,
			Capabilities:  capabilities,
		})
	}
	for _, raw := range items {
		item := jsonMap(raw)
		add(
			jsonString(item["model"]),
			firstNonEmpty(jsonString(item["displayName"]), jsonString(item["display_name"])),
			jsonPositiveIntPointer(firstNonNil(item["contextWindow"], item["context_window"])),
			jsonStringSlice(firstNonNil(item["inputModalities"], item["input_modalities"])),
		)
	}
	configuredModel = normalizeModelID(configuredModel)
	add(configuredModel, "", nil, nil)
	defaultModel := configuredModel
	if defaultModel == "" && len(models) > 0 {
		defaultModel = models[0].ModelID
	}
	return models, defaultModel
}

func normalizeCCSwitchModelID(raw string) (string, *int) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	longContext := ccSwitchLongContextSuffix.MatchString(trimmed)
	trimmed = strings.TrimSpace(ccSwitchLongContextSuffix.ReplaceAllString(trimmed, ""))
	if !longContext {
		return normalizeModelID(trimmed), nil
	}
	contextWindow := 1_000_000
	return normalizeModelID(trimmed), &contextWindow
}

func ccSwitchProviderKey(appType string, sourceID string, name string) string {
	identity := strings.TrimSpace(appType) + ":" + strings.TrimSpace(sourceID)
	digest := sha256.Sum256([]byte(identity))
	slug, _ := NormalizeProvider(firstNonEmpty(name, sourceID), true)
	if slug == "" {
		slug = "provider"
	}
	if len(slug) > 32 {
		slug = strings.Trim(slug[:32], "-")
	}
	return fmt.Sprintf("ccswitch-%s-%s-%x", appType, slug, digest[:4])
}

func ccSwitchAPIFormat(metaFormat string, wireAPI string) string {
	value := strings.ToLower(firstNonEmpty(metaFormat, wireAPI))
	switch value {
	case "", "anthropic", "anthropic_messages", "messages":
		return APIFormatAnthropicMessages
	case "chat", "openai_chat", "chat_completions", "openai_chat_completions":
		return APIFormatChatCompletions
	case "responses", "openai_responses":
		return APIFormatResponses
	default:
		return value
	}
}

func ccSwitchRuntimeSupport(apiFormat string) []string {
	switch strings.TrimSpace(apiFormat) {
	case APIFormatAnthropicMessages:
		return []string{"nxs", "claude"}
	case APIFormatChatCompletions, APIFormatResponses:
		return []string{"nxs"}
	default:
		return []string{}
	}
}

func ccSwitchRuntimeSupports(apiFormat string, runtimeKind string) bool {
	return stringSliceContainsFold(ccSwitchRuntimeSupport(apiFormat), normalizeRuntimeKind(runtimeKind))
}

func isOfficialAnthropicURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.anthropic.com")
}

func decodeJSONObject(raw string) map[string]any {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.UseNumber()
	var result map[string]any
	if decoder.Decode(&result) != nil || result == nil {
		return map[string]any{}
	}
	return result
}

func jsonMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func jsonSlice(value any) []any {
	result, _ := value.([]any)
	return result
}

func jsonString(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}

func jsonBool(value any) bool {
	result, _ := value.(bool)
	return result
}

func jsonPositiveIntPointer(value any) *int {
	var parsed int64
	switch current := value.(type) {
	case json.Number:
		parsed, _ = current.Int64()
	case float64:
		parsed = int64(current)
	case int:
		parsed = int64(current)
	case int64:
		parsed = current
	case string:
		parsed, _ = strconv.ParseInt(strings.TrimSpace(current), 10, 64)
	}
	if parsed <= 0 || parsed > int64(^uint(0)>>1) {
		return nil
	}
	result := int(parsed)
	return &result
}

func jsonStringSlice(value any) []string {
	items := jsonSlice(value)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := jsonString(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func jsonValueHasContent(value any) bool {
	switch current := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(current) != ""
	case map[string]any:
		for _, item := range current {
			if jsonValueHasContent(item) {
				return true
			}
		}
		return false
	case []any:
		return len(current) > 0
	default:
		return true
	}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func stringSliceContainsFold(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func isCCSwitchSourceMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
