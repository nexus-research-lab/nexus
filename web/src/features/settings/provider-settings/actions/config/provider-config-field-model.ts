import type {
  ProviderApiFormat,
  ProviderKind,
  ProviderPreset,
  ProviderPresetFormat,
} from "@/types/capability/provider";

import {
  DEFAULT_AGENT_API_FORMAT,
  formatSupportsProviderKind,
  getPresetFormat,
  getSupportedPresetFormat,
} from "../../model/provider-preset-model";
import type { ProviderDraft } from "../../model/provider-settings-types";

const DEFAULT_FORMAT_BY_PROVIDER_KIND: Record<
  ProviderKind,
  ProviderApiFormat
> = {
  image_generation: "chat_completions",
  llm: DEFAULT_AGENT_API_FORMAT,
};

type ProviderKindPatch = Pick<
  ProviderDraft,
  "provider_kind" | "api_format" | "base_url" | "models_path"
>;

type ProviderFormatPatch = Pick<
  ProviderDraft,
  "api_format" | "base_url" | "models_path"
>;

export type ProviderFormatTransition =
  | { kind: "reject" }
  | { kind: "update"; patch: ProviderFormatPatch };

export function buildProviderKindPatch(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  providerKind: ProviderKind,
): ProviderKindPatch {
  const format = resolveFormatForKind(draft, preset, providerKind);
  return {
    provider_kind: providerKind,
    ...projectFormatPatch(
      draft,
      format,
      DEFAULT_FORMAT_BY_PROVIDER_KIND[providerKind],
    ),
  };
}

export function buildProviderFormatTransition(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  apiFormat: ProviderApiFormat,
  canSelectNonRuntimeFormat: boolean,
): ProviderFormatTransition {
  const format = getPresetFormat(preset, apiFormat);
  if (!formatIsSelectable(format, draft.provider_kind, canSelectNonRuntimeFormat)) {
    return { kind: "reject" };
  }
  return {
    kind: "update",
    patch: projectFormatPatch(draft, format, apiFormat),
  };
}

function resolveFormatForKind(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  providerKind: ProviderKind,
): ProviderPresetFormat | null {
  const currentFormat = getPresetFormat(preset, draft.api_format);
  if (currentFormat && formatSupportsProviderKind(currentFormat, providerKind)) {
    return currentFormat;
  }
  return getSupportedPresetFormat(preset, providerKind);
}

function formatIsSelectable(
  format: ProviderPresetFormat | null,
  providerKind: ProviderKind,
  canSelectNonRuntimeFormat: boolean,
): boolean {
  return canSelectNonRuntimeFormat
    || Boolean(format && formatSupportsProviderKind(format, providerKind));
}

function projectFormatPatch(
  draft: ProviderDraft,
  format: ProviderPresetFormat | null,
  fallbackApiFormat: ProviderApiFormat,
): ProviderFormatPatch {
  return {
    api_format: format?.api_format ?? fallbackApiFormat,
    base_url: format?.base_url ?? draft.base_url,
    models_path: format?.models_path ?? draft.models_path,
  };
}
