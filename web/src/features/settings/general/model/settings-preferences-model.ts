import { getUserPreferences } from "@/config/runtime-options";
import {
  formatProviderOptionLabel,
  type ProviderOption,
} from "@/types/capability/provider";
import {
  normalizeAgentRuntimeKind,
  type UpdateUserPreferencesParams,
  type UserPreferences,
} from "@/types/settings/preferences";

export type DefaultModelPreferenceRole = "agent_runtime" | "image_generation" | "background_task";

export interface PreferenceFeedback {
  message: string;
}

export interface DefaultModelSelection {
  model: string;
  provider: string;
}

export function buildPreferencesUpdatePayload(
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

export function normalizePreferences(preferences: UserPreferences | null): UserPreferences {
  const fallback = getUserPreferences();
  return {
    chat_default_delivery_policy:
      preferences?.chat_default_delivery_policy ?? fallback.chat_default_delivery_policy,
    agent_runtime_kind: normalizeAgentRuntimeKind(
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
    default_image_model_selection: normalizeModelSelectionPreference(
      preferences?.default_image_model_selection ?? fallback.default_image_model_selection,
    ),
    default_background_model_selection: normalizeModelSelectionPreference(
      preferences?.default_background_model_selection ?? fallback.default_background_model_selection,
    ),
    updated_at: preferences?.updated_at,
  };
}

function normalizeModelSelectionPreference(
  selection: UserPreferences["default_image_model_selection"],
): UserPreferences["default_image_model_selection"] {
  const provider = selection?.provider?.trim();
  const model = selection?.model?.trim();
  if (!provider || !model) {
    return undefined;
  }
  return { provider, model };
}

function encodeDefaultModelValue(provider: string, model: string): string {
  return JSON.stringify([provider, model]);
}

export function decodeDefaultModelValue(value: string): { provider: string; model: string } | null {
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed) || parsed.length !== 2) {
      return null;
    }
    const [provider, model] = parsed;
    if (typeof provider !== "string" || typeof model !== "string") {
      return null;
    }
    const normalizedProvider = provider.trim();
    const normalizedModel = model.trim();
    if (!normalizedProvider || !normalizedModel) {
      return null;
    }
    return { provider: normalizedProvider, model: normalizedModel };
  } catch {
    return null;
  }
}

const DEFAULT_MODEL_UPDATERS: Record<
  DefaultModelPreferenceRole,
  (preferences: UserPreferences, selection: DefaultModelSelection) => UserPreferences
> = {
  agent_runtime: (preferences, selection) => ({
    ...preferences,
    default_agent_options: {
      ...preferences.default_agent_options,
      model: selection.model,
      provider: selection.provider,
    },
  }),
  image_generation: (preferences, selection) => ({
    ...preferences,
    default_image_model_selection: selection,
  }),
  background_task: (preferences, selection) => ({
    ...preferences,
    default_background_model_selection: selection,
  }),
};

export function applyDefaultModelSelection(
  preferences: UserPreferences,
  role: DefaultModelPreferenceRole,
  selection: DefaultModelSelection,
): UserPreferences {
  return normalizePreferences(DEFAULT_MODEL_UPDATERS[role](preferences, selection));
}

export function encodeOptionalModelSelection(
  provider?: string | null,
  model?: string | null,
): string {
  const normalizedProvider = provider?.trim();
  const normalizedModel = model?.trim();
  if (!normalizedProvider || !normalizedModel) {
    return "";
  }
  return encodeDefaultModelValue(normalizedProvider, normalizedModel);
}

export function buildDefaultModelOptions(
  providerOptions: ProviderOption[],
  subscriptionLabel: string,
) {
  return providerOptions.flatMap((provider) => (
    provider.models.map((model) => {
      const providerLabel = formatProviderOptionLabel(provider, subscriptionLabel);
      const modelLabel = model.display_name || model.model_id;
      return {
        value: encodeDefaultModelValue(provider.provider, model.model_id),
        label: `${providerLabel} / ${modelLabel}`,
      };
    })
  ));
}
