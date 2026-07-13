import { normalizeModelSelectionPreference } from "@/lib/settings/preferences-normalization";
import {
  formatProviderOptionLabel,
  type ProviderOption,
  type ProviderOptionsResponse,
} from "@/types/capability/provider";
import type { UserPreferences } from "@/types/settings/preferences";

import { normalizePreferences } from "./settings-preferences-model";

export type DefaultModelPreferenceRole =
  | "agent_runtime"
  | "background_task"
  | "image_generation";

export interface DefaultModelSelection {
  model: string;
  provider: string;
}

export interface DefaultModelCatalog {
  agentDefault: DefaultModelSelection | null;
  agentOptions: ProviderOption[];
  backgroundOptions: ProviderOption[];
  imageDefault: DefaultModelSelection | null;
  imageOptions: ProviderOption[];
}

export interface DefaultModelPreferencesView {
  options: {
    agent: ReturnType<typeof buildDefaultModelOptions>;
    background: ReturnType<typeof buildDefaultModelOptions>;
    image: ReturnType<typeof buildDefaultModelOptions>;
  };
  values: {
    agent: string;
    background: string;
    image: string;
  };
}

export const EMPTY_DEFAULT_MODEL_CATALOG: DefaultModelCatalog = {
  agentDefault: null,
  agentOptions: [],
  backgroundOptions: [],
  imageDefault: null,
  imageOptions: [],
};

function encodeDefaultModelValue(provider: string, model: string): string {
  return JSON.stringify([provider, model]);
}

export function decodeDefaultModelValue(
  value: string,
): DefaultModelSelection | null {
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!isDefaultModelTuple(parsed)) {
      return null;
    }
    const [provider, model] = parsed;
    return normalizeModelSelectionPreference({ provider, model }) ?? null;
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
  background_task: (preferences, selection) => ({
    ...preferences,
    default_background_model_selection: selection,
  }),
  image_generation: (preferences, selection) => ({
    ...preferences,
    default_image_model_selection: selection,
  }),
};

export function applyDefaultModelSelection(
  preferences: UserPreferences,
  role: DefaultModelPreferenceRole,
  selection: DefaultModelSelection,
): UserPreferences {
  return normalizePreferences(DEFAULT_MODEL_UPDATERS[role](preferences, selection));
}

function encodeOptionalModelSelection(
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

function buildDefaultModelOptions(
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

function buildDefaultModelSelection(
  provider: string | null,
  model: string | null,
): DefaultModelSelection | null {
  return normalizeModelSelectionPreference({
    provider: provider ?? undefined,
    model: model ?? undefined,
  }) ?? null;
}

export function buildDefaultModelCatalog(
  response: ProviderOptionsResponse,
): DefaultModelCatalog {
  return {
    agentDefault: buildDefaultModelSelection(
      response.default_provider,
      response.default_model,
    ),
    agentOptions: response.items,
    backgroundOptions: response.background_items,
    imageDefault: buildDefaultModelSelection(
      response.default_image_provider,
      response.default_image_model,
    ),
    imageOptions: response.image_items,
  };
}

function encodeModelSelection(
  selection?: DefaultModelSelection | null,
): string {
  return encodeOptionalModelSelection(selection?.provider, selection?.model);
}

function preferModelSelection(
  preferred: DefaultModelSelection | undefined,
  fallback: DefaultModelSelection | null,
): DefaultModelSelection | null {
  return preferred ?? fallback;
}

function buildDefaultModelValues(
  catalog: DefaultModelCatalog,
  preferences: UserPreferences,
): DefaultModelPreferencesView["values"] {
  const agentSelection = normalizeModelSelectionPreference({
    provider: preferences.default_agent_options.provider,
    model: preferences.default_agent_options.model,
  });
  const imageSelection = normalizeModelSelectionPreference(
    preferences.default_image_model_selection,
  );
  const backgroundSelection = normalizeModelSelectionPreference(
    preferences.default_background_model_selection,
  );
  return {
    agent: encodeModelSelection(preferModelSelection(
      agentSelection,
      catalog.agentDefault,
    )),
    background: encodeModelSelection(backgroundSelection),
    image: encodeModelSelection(preferModelSelection(
      imageSelection,
      catalog.imageDefault,
    )),
  };
}

export function buildDefaultModelPreferencesView(
  catalog: DefaultModelCatalog,
  preferences: UserPreferences,
  subscriptionLabel: string,
): DefaultModelPreferencesView {
  return {
    options: {
      agent: buildDefaultModelOptions(
        catalog.agentOptions,
        subscriptionLabel,
      ),
      background: buildDefaultModelOptions(
        catalog.backgroundOptions,
        subscriptionLabel,
      ),
      image: buildDefaultModelOptions(
        catalog.imageOptions,
        subscriptionLabel,
      ),
    },
    values: buildDefaultModelValues(catalog, preferences),
  };
}
