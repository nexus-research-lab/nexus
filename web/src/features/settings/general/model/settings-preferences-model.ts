/**
 * INPUT: 服务端 Preferences、本页草稿与对账后的最新版本。
 * OUTPUT: 规范化偏好、PATCH 正文、叶子变更重应用与完整恢复反馈类型。
 * POS: Preferences 纯模型边界；不访问 React、HTTP 或全局 runtime 状态写入。
 */
import { getUserPreferences } from "@/config/runtime-options";
import {
  mergeAgentOptions,
  normalizeModelSelectionPreference,
} from "@/lib/settings/preferences-normalization";
import {
  normalizeAgentRuntimeKind,
  DEFAULT_WEB_SEARCH_PROVIDER,
  type UpdateUserPreferencesParams,
  type UserPreferences,
} from "@/types/settings/preferences";

export interface PreferenceFeedback {
  impact: string;
  message: string;
  nextStep: string;
  title: string;
  tone: "error" | "success" | "warning";
}

export interface PreferenceRecoveryControls {
  canCompare: boolean;
  canRepairProjection: boolean;
  checking: boolean;
  checkLatest: () => void;
  repairProjection: () => void;
  reapplyDraft: () => void;
  repairing: boolean;
  useLatest: () => void;
}

export function buildPreferencesUpdatePayload(
  preferences: UserPreferences,
): UpdateUserPreferencesParams {
  return {
    chat_default_delivery_policy: preferences.chat_default_delivery_policy,
    agent_runtime_kind: preferences.agent_runtime_kind,
    agent_sdk_diagnostics_enabled: preferences.agent_sdk_diagnostics_enabled,
    emotion_enabled: preferences.emotion_enabled,
    browser_cdp_enabled: preferences.browser_cdp_enabled,
    runtime_settings: preferences.runtime_settings,
    web_search: preferences.web_search,
    web_search_api_key: preferences.web_search_api_key,
    default_agent_options: preferences.default_agent_options,
    default_image_model_selection: preferences.default_image_model_selection,
    default_vision_model_selection: preferences.default_vision_model_selection,
    default_background_model_selection:
      preferences.default_background_model_selection,
  };
}

export function normalizePreferences(preferences: UserPreferences | null): UserPreferences {
  const fallback = getUserPreferences();
  const source: Partial<UserPreferences> = preferences ?? {};
  return {
    version: normalizePreferencesVersion(source.version),
    chat_default_delivery_policy: preferDefined(
      source.chat_default_delivery_policy,
      fallback.chat_default_delivery_policy,
    ),
    agent_runtime_kind: normalizeAgentRuntimeKind(
      preferDefined(source.agent_runtime_kind, fallback.agent_runtime_kind),
    ),
    agent_sdk_diagnostics_enabled: resolveDiagnosticsEnabled(
      preferences,
      fallback,
    ),
    emotion_enabled: resolveEmotionEnabled(preferences, fallback),
    browser_cdp_enabled: resolveBrowserCDPEnabled(preferences, fallback),
    echo_enabled: preferences === null
      ? fallback.echo_enabled === true
      : preferences.echo_enabled === true,
    runtime_settings: normalizeRuntimeSettings(
      source.runtime_settings,
      fallback.runtime_settings,
    ),
    web_search: normalizeWebSearch(source.web_search, fallback.web_search),
    web_search_api_key: source.web_search_api_key,
    default_agent_options: mergeAgentOptions(
      fallback.default_agent_options,
      source.default_agent_options,
    ),
    default_image_model_selection: normalizeModelSelectionPreference(
      preferDefined(
        source.default_image_model_selection,
        fallback.default_image_model_selection,
      ),
    ),
    default_vision_model_selection: normalizeModelSelectionPreference(
      preferDefined(
        source.default_vision_model_selection,
        fallback.default_vision_model_selection,
      ),
    ),
    default_background_model_selection: normalizeModelSelectionPreference(
      preferDefined(
        source.default_background_model_selection,
        fallback.default_background_model_selection,
      ),
    ),
    updated_at: source.updated_at,
  };
}

/**
 * 把本页相对旧基线的叶子变更叠加到最新服务端版本。
 * version/updated_at 始终使用最新服务端值，不得被草稿覆盖。
 */
export function rebasePreferenceDraft(
  base: UserPreferences,
  draft: UserPreferences,
  latest: UserPreferences,
): UserPreferences {
  const rebased = applyPreferenceDelta(
    latest as unknown as JsonObject,
    createPreferenceDelta(
      base as unknown as JsonObject,
      draft as unknown as JsonObject,
    ),
  ) as unknown as UserPreferences;
  rebased.version = latest.version;
  rebased.updated_at = latest.updated_at;
  return normalizePreferences(rebased);
}

export function equivalentPreferences(
  left: UserPreferences,
  right: UserPreferences,
): boolean {
  if (left.web_search_api_key !== undefined || right.web_search_api_key !== undefined) {
    return false;
  }
  const leftComparable = comparablePreferences(left);
  const rightComparable = comparablePreferences(right);
  return equalJsonValue(leftComparable, rightComparable);
}

type JsonObject = Record<string, unknown>;
type PreferenceDelta =
  | { kind: "delete" }
  | { kind: "object"; values: Record<string, PreferenceDelta> }
  | { kind: "replace"; value: unknown }
  | null;

function createPreferenceDelta(base: JsonObject, draft: JsonObject): PreferenceDelta {
  const values: Record<string, PreferenceDelta> = {};
  const keys = new Set([...Object.keys(base), ...Object.keys(draft)]);
  keys.delete("updated_at");
  keys.delete("version");
  for (const key of keys) {
    const delta = createValueDelta(base[key], draft[key]);
    if (delta) {
      values[key] = delta;
    }
  }
  return Object.keys(values).length > 0 ? { kind: "object", values } : null;
}

function createValueDelta(base: unknown, draft: unknown): PreferenceDelta {
  if (equalJsonValue(base, draft)) {
    return null;
  }
  if (draft === undefined) {
    return { kind: "delete" };
  }
  if (isJsonObject(base) && isJsonObject(draft)) {
    const values: Record<string, PreferenceDelta> = {};
    const keys = new Set([...Object.keys(base), ...Object.keys(draft)]);
    for (const key of keys) {
      const delta = createValueDelta(base[key], draft[key]);
      if (delta) {
        values[key] = delta;
      }
    }
    return Object.keys(values).length > 0 ? { kind: "object", values } : null;
  }
  return { kind: "replace", value: cloneJsonValue(draft) };
}

function applyPreferenceDelta(source: JsonObject, delta: PreferenceDelta): JsonObject {
  const result = cloneJsonValue(source) as JsonObject;
  if (!delta || delta.kind !== "object") {
    return result;
  }
  for (const [key, value] of Object.entries(delta.values)) {
    if (value?.kind === "delete") {
      delete result[key];
      continue;
    }
    if (value?.kind === "object") {
      result[key] = applyPreferenceDelta(
        isJsonObject(result[key]) ? result[key] : {},
        value,
      );
      continue;
    }
    if (value?.kind === "replace") {
      result[key] = cloneJsonValue(value.value);
    }
  }
  return result;
}

function comparablePreferences(value: UserPreferences): JsonObject {
  const comparable = cloneJsonValue(value as unknown as JsonObject) as JsonObject;
  delete comparable.updated_at;
  delete comparable.version;
  // GET 不返回原始密钥，因此不能仅凭读响应证明密钥草稿已提交。
  delete comparable.web_search_api_key;
  return comparable;
}

function equalJsonValue(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) {
    return true;
  }
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left)
      && Array.isArray(right)
      && left.length === right.length
      && left.every((value, index) => equalJsonValue(value, right[index]));
  }
  if (!isJsonObject(left) || !isJsonObject(right)) {
    return false;
  }
  const leftKeys = Object.keys(left).filter((key) => left[key] !== undefined).sort();
  const rightKeys = Object.keys(right).filter((key) => right[key] !== undefined).sort();
  return leftKeys.length === rightKeys.length
    && leftKeys.every((key, index) => (
      key === rightKeys[index] && equalJsonValue(left[key], right[key])
    ));
}

function isJsonObject(value: unknown): value is JsonObject {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function cloneJsonValue<T>(value: T): T {
  if (Array.isArray(value)) {
    return value.map((item) => cloneJsonValue(item)) as T;
  }
  if (isJsonObject(value)) {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, cloneJsonValue(item)]),
    ) as T;
  }
  return value;
}

function normalizePreferencesVersion(value: number | undefined): number | undefined {
  return Number.isSafeInteger(value) && (value ?? 0) > 0 ? value : undefined;
}

function normalizeWebSearch(
  settings: UserPreferences["web_search"],
  fallback: UserPreferences["web_search"],
): UserPreferences["web_search"] {
  const apiKeyConfigured = settings?.api_key_configured === true;
  return {
    ...fallback,
    ...settings,
    enabled: settings?.enabled ?? fallback?.enabled ?? true,
    provider: settings?.provider ?? fallback?.provider ?? DEFAULT_WEB_SEARCH_PROVIDER,
    api_key_configured: apiKeyConfigured,
    api_key_masked: apiKeyConfigured ? settings?.api_key_masked ?? "" : "",
  };
}

function normalizeRuntimeSettings(
  settings: UserPreferences["runtime_settings"],
  fallback: UserPreferences["runtime_settings"],
): UserPreferences["runtime_settings"] {
  return {
    ...fallback,
    ...settings,
    nxs: {
      ...fallback?.nxs,
      ...settings?.nxs,
      auto_memory_enabled: settings?.nxs?.auto_memory_enabled
        ?? fallback?.nxs?.auto_memory_enabled
        ?? true,
      auto_dream_enabled: settings?.nxs?.auto_dream_enabled
        ?? fallback?.nxs?.auto_dream_enabled
        ?? true,
      tool_search: settings?.nxs?.tool_search
        ?? fallback?.nxs?.tool_search
        ?? false,
    },
  };
}

function preferDefined<T>(preferred: T | undefined, fallback: T): T {
  return preferred ?? fallback;
}

function resolveDiagnosticsEnabled(
  preferences: UserPreferences | null,
  fallback: UserPreferences,
): boolean {
  if (preferences === null) {
    return fallback.agent_sdk_diagnostics_enabled === true;
  }
  return preferences.agent_sdk_diagnostics_enabled === true;
}

function resolveEmotionEnabled(
  preferences: UserPreferences | null,
  fallback: UserPreferences,
): boolean {
  if (preferences === null) {
    return fallback.emotion_enabled === true;
  }
  return preferences.emotion_enabled === true;
}

function resolveBrowserCDPEnabled(
  preferences: UserPreferences | null,
  fallback: UserPreferences,
): boolean {
  if (preferences === null) {
    return fallback.browser_cdp_enabled === true;
  }
  return preferences.browser_cdp_enabled === true;
}
