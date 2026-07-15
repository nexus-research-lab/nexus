import { getUserPreferences } from "@/config/runtime-options";
import {
  mergeAgentOptions,
  normalizeModelSelectionPreference,
} from "@/lib/settings/preferences-normalization";
import {
  normalizeAgentRuntimeKind,
  type UpdateUserPreferencesParams,
  type UserPreferences,
} from "@/types/settings/preferences";

export interface PreferenceFeedback {
  message: string;
}

export function buildPreferencesUpdatePayload(
  preferences: UserPreferences,
): UpdateUserPreferencesParams {
  return {
    chat_default_delivery_policy: preferences.chat_default_delivery_policy,
    agent_runtime_kind: preferences.agent_runtime_kind,
    agent_sdk_diagnostics_enabled: preferences.agent_sdk_diagnostics_enabled,
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

function normalizeWebSearch(
  settings: UserPreferences["web_search"],
  fallback: UserPreferences["web_search"],
): UserPreferences["web_search"] {
  return {
    ...fallback,
    ...settings,
    enabled: settings?.enabled === true,
    provider: settings?.provider ?? fallback?.provider ?? "brave",
    default_count: settings?.default_count ?? fallback?.default_count ?? 5,
    timeout_seconds: settings?.timeout_seconds ?? fallback?.timeout_seconds ?? 20,
    cache_ttl_seconds: settings?.cache_ttl_seconds ?? fallback?.cache_ttl_seconds ?? 900,
    search_depth: settings?.search_depth ?? fallback?.search_depth ?? "basic",
    extract_depth: settings?.extract_depth ?? fallback?.extract_depth ?? "basic",
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
