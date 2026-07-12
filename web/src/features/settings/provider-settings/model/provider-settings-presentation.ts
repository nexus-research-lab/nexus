import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderKind,
  ProviderPreset,
} from "@/types/capability/provider";

import { buildProviderCatalog } from "./provider-catalog-model";
import {
  getEffectiveModelsPath,
  getProviderTitle,
} from "./provider-config-model";
import {
  formatSupportsProviderKind,
  getPresetFormat,
  orderedPresetProviderKinds,
  presetUsesBuiltinEndpoint,
  SUPPORTED_AGENT_API_FORMATS,
  uniquePresetFormats,
} from "./provider-preset-model";
import type { ProviderDraft } from "./provider-settings-types";

export type SettingsTabKey = "providers";

export const SETTINGS_TABS: {
  key: SettingsTabKey;
  labelKey: "settings.tabs.providers";
}[] = [
  { key: "providers", labelKey: "settings.tabs.providers" },
];

export const PROVIDER_LABEL_CLASS_NAME =
  "text-[13px] font-semibold text-(--text-strong)";

export const API_FORMAT_LABELS: Record<ProviderApiFormat, string> = {
  chat_completions: "Chat Completions (/chat/completions)",
  responses: "Responses (/responses)",
  anthropic_messages: "Anthropic Messages (/v1/messages)",
  openai_image_generation: "OpenAI Image Generation (/images/generations)",
  dashscope_image_generation: "DashScope Image Generation",
  modelscope_image_generation: "ModelScope Image Generation",
};

export const API_FORMAT_SHORT_LABELS: Record<ProviderApiFormat, string> = {
  chat_completions: "Completions",
  responses: "Responses",
  anthropic_messages: "Anthropic",
  openai_image_generation: "OpenAI Image",
  dashscope_image_generation: "DashScope Image",
  modelscope_image_generation: "ModelScope Image",
};

const PROVIDER_KIND_LABEL_KEYS = {
  image_generation: "settings.providers.kind_image_generation",
  llm: "settings.providers.kind_llm",
} as const;

interface BuildProviderSettingsPresentationOptions {
  canSelectNonRuntimeFormat: boolean;
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  isEditing: boolean;
  presets: ProviderPreset[];
  providers: ProviderConfigRecord[];
  selectedRecord: ProviderConfigRecord | null;
  t: I18nContextValue["t"];
}

function buildProviderKindOptions(
  preset: ProviderPreset | null,
  t: I18nContextValue["t"],
) {
  return orderedPresetProviderKinds(preset).map((kind: ProviderKind) => ({
    value: kind,
    label: t(PROVIDER_KIND_LABEL_KEYS[kind]),
  }));
}

function buildProviderFormatOptions(
  preset: ProviderPreset | null,
  providerKind: ProviderKind,
  canSelectNonRuntimeFormat: boolean,
  unsupportedSuffix: string,
) {
  return uniquePresetFormats(preset).map((format) => {
    const supported = formatSupportsProviderKind(format, providerKind);
    const selectable = supported || canSelectNonRuntimeFormat;
    return {
      value: format.api_format,
      label: selectable
        ? API_FORMAT_LABELS[format.api_format]
        : `${API_FORMAT_LABELS[format.api_format]}${unsupportedSuffix}`,
      disabled: !selectable,
    };
  });
}

function resolveBuiltinEndpointFormats(
  preset: ProviderPreset | null,
  usesBuiltinEndpoint: boolean,
) {
  return usesBuiltinEndpoint ? preset?.formats ?? [] : [];
}

function resolveProviderDetailTitle({
  currentPreset,
  draft,
  isEditing,
  selectedRecord,
  t,
}: Pick<
  BuildProviderSettingsPresentationOptions,
  "currentPreset" | "draft" | "isEditing" | "selectedRecord" | "t"
>): string {
  if (isEditing && selectedRecord) {
    return getProviderTitle(selectedRecord);
  }
  return draft.display_name
    || currentPreset?.display_name
    || t("settings.providers.custom_provider");
}

function isProviderFormatConfigurable(
  format: ReturnType<typeof getPresetFormat>,
  providerKind: ProviderKind,
  canSelectNonRuntimeFormat: boolean,
): boolean {
  return canSelectNonRuntimeFormat
    || Boolean(format && formatSupportsProviderKind(format, providerKind));
}

function shouldShowRuntimeFormatBadge(draft: ProviderDraft): boolean {
  return draft.provider_kind === "llm"
    && !SUPPORTED_AGENT_API_FORMATS.has(draft.api_format);
}

export function buildProviderSettingsPresentation({
  canSelectNonRuntimeFormat,
  currentPreset,
  draft,
  isEditing,
  presets,
  providers,
  selectedRecord,
  t,
}: BuildProviderSettingsPresentationOptions) {
  const catalog = buildProviderCatalog(providers);
  const usesBuiltinEndpoint = presetUsesBuiltinEndpoint(currentPreset);
  const currentFormat = getPresetFormat(currentPreset, draft.api_format);
  return {
    builtinEndpointFormats: resolveBuiltinEndpointFormats(
      currentPreset,
      usesBuiltinEndpoint,
    ),
    configuredByPreset: catalog.configuredByPreset,
    currentFormat,
    customProviders: catalog.customProviders,
    detailTitle: resolveProviderDetailTitle({
      currentPreset,
      draft,
      isEditing,
      selectedRecord,
      t,
    }),
    formatOptions: buildProviderFormatOptions(
      currentPreset,
      draft.provider_kind,
      canSelectNonRuntimeFormat,
      t("settings.providers.unsupported_suffix"),
    ),
    hasModelsEndpoint: Boolean(
      getEffectiveModelsPath(draft, currentPreset).trim(),
    ),
    isApiFormatConfigurable: isProviderFormatConfigurable(
      currentFormat,
      draft.provider_kind,
      canSelectNonRuntimeFormat,
    ),
    presetSidebarItems: presets.filter(
      (preset) => preset.preset_key !== "custom",
    ),
    providerKindOptions: buildProviderKindOptions(currentPreset, t),
    showProviderShapeControls: draft.preset_key === "custom",
    showRuntimeFormatBadge: shouldShowRuntimeFormatBadge(draft),
    usesBuiltinEndpoint,
  };
}

export function formatTokenPreview(
  maskedToken: string | null | undefined,
  emptyLabel: string,
): string {
  return maskedToken?.trim() || emptyLabel;
}

export function formatCount(value?: number | null): string {
  if (!value || value <= 0) {
    return "auto";
  }
  if (value >= 1000) {
    return `${Math.round(value / 1000)}K`;
  }
  return String(value);
}
