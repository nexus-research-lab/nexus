import { getUserPreferences } from "@/config/runtime-options";
import {
  mergeAgentOptions,
  normalizeModelSelectionPreference,
} from "@/lib/settings/preferences-normalization";
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
    default_background_model_selection: normalizeModelSelectionPreference(
      preferDefined(
        source.default_background_model_selection,
        fallback.default_background_model_selection,
      ),
    ),
    updated_at: source.updated_at,
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

function encodeDefaultModelValue(provider: string, model: string): string {
  return JSON.stringify([provider, model]);
}

export function decodeDefaultModelValue(value: string): { provider: string; model: string } | null {
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!isDefaultModelTuple(parsed)) {
      return null;
    }
    const [provider, model] = parsed;
    const selection = normalizeModelSelectionPreference({ provider, model });
    return selection ?? null;
  } catch {
    return null;
  }
}

function isDefaultModelTuple(value: unknown): value is [string, string] {
  return Array.isArray(value)
    && value.length === 2
    && value.every((item) => typeof item === "string");
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
