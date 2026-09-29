// INPUT: Available provider options and saved model preferences.
// OUTPUT: Eligible default choices; unavailable saved vision choices have a visible warning.
// POS: Default-model presentation; never silently rewrites saved selection intent.
import { modelGuidanceLabel, recommendedModelsFirst } from "@/entities/provider/model-guidance";
import type { ModelPurpose } from "@/types/capability/provider";
import type { useI18n } from "@/shared/i18n/i18n-context";
import { normalizeModelSelectionPreference } from "@/lib/settings/preferences-normalization";
import {
  formatProviderOptionLabel,
  type ProviderOption,
  type ProviderOptionsResponse,
} from "@/types/capability/provider";
import type { UserPreferences } from "@/types/settings/preferences";

import { normalizePreferences } from "../general/model/settings-preferences-model";

export type DefaultModelPreferenceRole =
  | "agent_runtime"
  | "background_task"
  | "image_generation"
  | "vision_understanding";

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
  visionOptions: ProviderOption[];
}

export interface DefaultModelPreferencesView {
  // 已保存选择不在可用目录中时单独提示，不把失效项放回选择器。
  unavailableVisionSelection: string | null;
  options: {
    agent: ReturnType<typeof buildDefaultModelOptions>;
    background: ReturnType<typeof buildDefaultModelOptions>;
    image: ReturnType<typeof buildDefaultModelOptions>;
    vision: ReturnType<typeof buildDefaultModelOptions>;
  };
  values: {
    agent: string;
    background: string;
    image: string;
    vision: string;
  };
}

export const EMPTY_DEFAULT_MODEL_CATALOG: DefaultModelCatalog = {
  agentDefault: null,
  agentOptions: [],
  backgroundOptions: [],
  imageDefault: null,
  imageOptions: [],
  visionOptions: [],
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
  vision_understanding: (preferences, selection) => ({
    ...preferences,
    default_vision_model_selection: selection,
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
  purpose: ModelPurpose,
  t?: ReturnType<typeof useI18n>["t"],
) {
  return providerOptions.flatMap((provider) => (
    recommendedModelsFirst(provider.models, purpose).map((model) => {
      const providerLabel = formatProviderOptionLabel(provider, subscriptionLabel);
      const modelLabel = model.display_name || model.model_id;
      return {
        value: encodeDefaultModelValue(provider.provider, model.model_id),
        label: [`${providerLabel} / ${modelLabel}`, t ? modelGuidanceLabel(model.guidance, purpose, t) : ""].filter(Boolean).join(" · "),
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
    visionOptions: response.vision_items,
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
  const visionSelection = normalizeModelSelectionPreference(
    preferences.default_vision_model_selection,
  );
  const values = {
    agent: encodeModelSelection(preferModelSelection(
      agentSelection,
      catalog.agentDefault,
    )),
    background: encodeModelSelection(backgroundSelection),
    image: encodeModelSelection(preferModelSelection(
      imageSelection,
      catalog.imageDefault,
    )),
    vision: encodeModelSelection(visionSelection),
  };
  const available: Record<keyof typeof values, ProviderOption[]> = {
    agent: catalog.agentOptions, background: catalog.backgroundOptions,
    image: catalog.imageOptions, vision: catalog.visionOptions,
  };
  for (const role of Object.keys(values) as Array<keyof typeof values>) {
    const selectable = available[role].some((provider) => provider.models.some((model) =>
      encodeDefaultModelValue(provider.provider, model.model_id) === values[role]));
    if (!selectable) values[role] = "";
  }
  return values;
}

export function buildDefaultModelPreferencesView(
  catalog: DefaultModelCatalog,
  preferences: UserPreferences,
  subscriptionLabel: string,
  t?: ReturnType<typeof useI18n>["t"],
): DefaultModelPreferencesView {
  const values = buildDefaultModelValues(catalog, preferences);
  const savedVision = preferences.default_vision_model_selection;
  const savedVisionLabel = [savedVision?.provider?.trim(), savedVision?.model?.trim()]
    .filter(Boolean).join(" / ");
  return {
    unavailableVisionSelection: savedVisionLabel && !values.vision ? savedVisionLabel : null,
    options: {
      agent: buildDefaultModelOptions(
        catalog.agentOptions,
        subscriptionLabel,
        "chat", t,
      ),
      background: buildDefaultModelOptions(
        catalog.backgroundOptions,
        subscriptionLabel,
        "chat", t,
      ),
      image: buildDefaultModelOptions(
        catalog.imageOptions,
        subscriptionLabel,
        "image_generation", t,
      ),
      vision: buildDefaultModelOptions(
        catalog.visionOptions,
        subscriptionLabel,
        "vision", t,
      ),
    },
    values,
  };
}
