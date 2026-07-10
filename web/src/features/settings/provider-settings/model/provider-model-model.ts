import type {
  ProviderModelCapabilities,
  ProviderModelRecord,
  UpdateProviderModelPayload,
} from "@/types/capability/provider";

import type { ModelOptionsState } from "./provider-settings-types";

export const AUTO_TEST_MODEL_VALUE = "__auto__";

export function parseProviderOptions(
  raw: string,
  invalidObjectMessage: string,
): Record<string, unknown> {
  const trimmed = raw.trim();
  if (!trimmed) {
    return {};
  }
  const parsed: unknown = JSON.parse(trimmed);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(invalidObjectMessage);
  }
  return parsed as Record<string, unknown>;
}

export function getEffectiveCapabilities(
  model: ProviderModelRecord,
): ProviderModelCapabilities {
  return {
    ...model.capabilities_auto,
    ...model.capabilities_override,
  };
}

export function sortModelsEnabledFirst(
  models: ProviderModelRecord[],
): ProviderModelRecord[] {
  return [...models].sort((left, right) => {
    if (left.enabled !== right.enabled) {
      return left.enabled ? -1 : 1;
    }
    return (left.display_name || left.model_id).localeCompare(
      right.display_name || right.model_id,
      "zh-Hans-CN",
    );
  });
}

export function filterProviderModels(
  models: ProviderModelRecord[],
  rawQuery: string,
): ProviderModelRecord[] {
  const query = rawQuery.trim().toLowerCase();
  if (!query) {
    return models;
  }
  return models.filter((model) => [
    model.model_id,
    model.display_name,
    model.category,
  ].some((value) => value.toLowerCase().includes(query)));
}

export function buildTestModelOptions(
  models: ProviderModelRecord[],
  autoLabel: string,
): { value: string; label: string }[] {
  return [
    { value: AUTO_TEST_MODEL_VALUE, label: autoLabel },
    ...sortModelsEnabledFirst(models).map((model) => {
      const displayName = model.display_name || model.model_id;
      return {
        value: model.model_id,
        label: displayName === model.model_id
          ? model.model_id
          : `${displayName} · ${model.model_id}`,
      };
    }),
  ];
}

export function modelOptionsFromRecord(
  model: ProviderModelRecord,
): ModelOptionsState {
  return {
    model,
    capabilities: { ...model.capabilities_override },
    context_window: model.context_window ? String(model.context_window) : "",
    max_output_tokens: model.max_output_tokens
      ? String(model.max_output_tokens)
      : "",
    provider_options_text: JSON.stringify(model.provider_options ?? {}, null, 2),
  };
}

export function modelUpdatePayload(
  model: ProviderModelRecord,
  override?: Partial<UpdateProviderModelPayload>,
): UpdateProviderModelPayload {
  return {
    enabled: model.enabled,
    is_default: model.is_default,
    capabilities_override: model.capabilities_override ?? {},
    context_window: model.context_window ?? null,
    max_output_tokens: model.max_output_tokens ?? null,
    provider_options: model.provider_options ?? {},
    ...override,
  };
}
