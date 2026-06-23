import { get_user_preferences } from "@/config/options";
import type { ProviderOption } from "@/types/capability/provider";
import {
  normalize_agent_runtime_kind,
  type UpdateUserPreferencesParams,
  type UserPreferences,
} from "@/types/settings/preferences";

export type DefaultModelPreferenceRole = "agent_runtime" | "image_generation" | "background_task";

export interface PreferenceFeedback {
  message: string;
}

export function build_preferences_update_payload(
  preferences: UserPreferences,
): UpdateUserPreferencesParams {
  return {
    chat_default_delivery_policy: preferences.chat_default_delivery_policy,
    agent_runtime_kind: preferences.agent_runtime_kind,
    agent_sdk_diagnostics_enabled: preferences.agent_sdk_diagnostics_enabled,
    default_agent_options: preferences.default_agent_options,
    default_image_model_selection: preferences.default_image_model_selection,
    default_background_model_selection:
      preferences.default_background_model_selection,
  };
}

export function normalize_preferences(preferences: UserPreferences | null): UserPreferences {
  const fallback = get_user_preferences();
  return {
    chat_default_delivery_policy:
      preferences?.chat_default_delivery_policy ?? fallback.chat_default_delivery_policy,
    agent_runtime_kind: normalize_agent_runtime_kind(
      preferences?.agent_runtime_kind ?? fallback.agent_runtime_kind,
    ),
    agent_sdk_diagnostics_enabled: preferences === null
      ? fallback.agent_sdk_diagnostics_enabled === true
      : preferences.agent_sdk_diagnostics_enabled === true,
    default_agent_options: {
      ...fallback.default_agent_options,
      ...(preferences?.default_agent_options ?? {}),
      allowed_tools: [
        ...(preferences?.default_agent_options?.allowed_tools ??
          fallback.default_agent_options.allowed_tools ??
          []),
      ],
      disallowed_tools: [
        ...(preferences?.default_agent_options?.disallowed_tools ??
          fallback.default_agent_options.disallowed_tools ??
          []),
      ],
      setting_sources: [
        ...(preferences?.default_agent_options?.setting_sources ??
          fallback.default_agent_options.setting_sources ??
          ["project"]),
      ],
    },
    default_image_model_selection: normalize_model_selection_preference(
      preferences?.default_image_model_selection ?? fallback.default_image_model_selection,
    ),
    default_background_model_selection: normalize_model_selection_preference(
      preferences?.default_background_model_selection ?? fallback.default_background_model_selection,
    ),
    updated_at: preferences?.updated_at,
  };
}

function normalize_model_selection_preference(
  selection: UserPreferences["default_image_model_selection"],
): UserPreferences["default_image_model_selection"] {
  const provider = selection?.provider?.trim();
  const model = selection?.model?.trim();
  if (!provider || !model) {
    return undefined;
  }
  return { provider, model };
}

export function encode_default_model_value(provider: string, model: string): string {
  return JSON.stringify([provider, model]);
}

export function decode_default_model_value(value: string): { provider: string; model: string } | null {
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed) || parsed.length !== 2) {
      return null;
    }
    const [provider, model] = parsed;
    if (typeof provider !== "string" || typeof model !== "string") {
      return null;
    }
    const normalized_provider = provider.trim();
    const normalized_model = model.trim();
    if (!normalized_provider || !normalized_model) {
      return null;
    }
    return { provider: normalized_provider, model: normalized_model };
  } catch {
    return null;
  }
}

export function encode_optional_model_selection(
  provider?: string | null,
  model?: string | null,
): string {
  const normalized_provider = provider?.trim();
  const normalized_model = model?.trim();
  if (!normalized_provider || !normalized_model) {
    return "";
  }
  return encode_default_model_value(normalized_provider, normalized_model);
}

export function build_default_model_options(provider_options: ProviderOption[]) {
  return provider_options.flatMap((provider) => (
    provider.models.map((model) => {
      const provider_label = provider.display_name || provider.provider;
      const model_label = model.display_name || model.model_id;
      return {
        value: encode_default_model_value(provider.provider, model.model_id),
        label: `${provider_label} / ${model_label}`,
      };
    })
  ));
}
