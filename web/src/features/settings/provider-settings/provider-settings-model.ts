import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderKind,
  ProviderModelCapabilities,
  ProviderModelRecord,
  ProviderPreset,
  ProviderPresetFormat,
  UpdateProviderConfigPayload,
  UpdateProviderModelPayload,
} from "@/types/capability/provider";

export type SettingsTabKey = "providers";
type FeedbackTone = "success" | "error";
export type FormMode = "empty" | "create" | "edit";

export interface FeedbackState {
  tone: FeedbackTone;
  title: string;
  message: string;
}

export interface ProviderDraft {
  provider_kind: ProviderKind;
  provider: string;
  preset_key: string;
  api_format: ProviderApiFormat;
  display_name: string;
  auth_token: string;
  base_url: string;
  models_path: string;
  enabled: boolean;
}

export interface ModelOptionsState {
  model: ProviderModelRecord;
  capabilities: ProviderModelCapabilities;
  context_window: string;
  max_output_tokens: string;
  provider_options_text: string;
}

export const SETTINGS_TABS: { key: SettingsTabKey; labelKey: "settings.tabs.providers" }[] = [
  { key: "providers", labelKey: "settings.tabs.providers" },
];

export const PROVIDER_LABEL_CLASS_NAME = "text-[13px] font-semibold text-(--text-strong)";

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

export const AUTO_TEST_MODEL_VALUE = "__auto__";
export const DEFAULT_AGENT_API_FORMAT: ProviderApiFormat = "anthropic_messages";
export const SUPPORTED_AGENT_API_FORMATS = new Set<ProviderApiFormat>([
  "anthropic_messages",
  "chat_completions",
]);

const SUPPORTED_IMAGE_API_FORMATS = new Set<ProviderApiFormat>([
  "chat_completions",
  "openai_image_generation",
  "dashscope_image_generation",
  "modelscope_image_generation",
]);
const CONFIGURABLE_NON_RUNTIME_PRESET_KEYS = new Set(["custom", "openai"]);

const PRESET_PROVIDER_KEYS: Record<string, string> = {
  anthropic: "anthropic",
  openai: "openai",
  deepseek: "deepseek",
  "qwen-token-plan": "qwen-token-plan",
  "minimax-token-plan": "minimax-token-plan",
  "glm-coding-plan": "glm-coding-plan",
  "kimi-code": "kimi-code",
  "volcengine-coding-plan": "volcengine-coding-plan",
  doubao: "doubao",
  dashscope: "dashscope",
  modelscope: "modelscope",
  azure: "azure",
};

function getPresetProviderKey(preset: ProviderPreset): string {
  return PRESET_PROVIDER_KEYS[preset.preset_key] ?? "";
}

export function getPresetFormat(preset: ProviderPreset | null, apiFormat?: ProviderApiFormat): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const targetFormat = apiFormat ?? preset.default_api_format;
  return preset.formats.find((item) => item.api_format === targetFormat) ?? preset.formats[0] ?? null;
}

export function formatSupportsProviderKind(format: ProviderPresetFormat, providerKind: ProviderKind): boolean {
  if (format.provider_kind) {
    return format.provider_kind === providerKind;
  }
  return apiFormatSupportedForKind(providerKind, format.api_format);
}

export function getSupportedPresetFormat(preset: ProviderPreset | null, providerKind?: ProviderKind): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const targetKind = providerKind ?? preset.provider_kind;
  const explicitFormat = preset.formats.find((item) => item.provider_kind === targetKind);
  if (explicitFormat) {
    return explicitFormat;
  }
  return preset.formats.find((item) => formatSupportsProviderKind(item, targetKind)) ?? null;
}

export function presetProviderKinds(preset: ProviderPreset | null): ProviderKind[] {
  if (!preset) {
    return [];
  }
  const values = new Set<ProviderKind>();
  for (const format of preset.formats) {
    if (format.provider_kind) {
      values.add(format.provider_kind);
      continue;
    }
    values.add("llm");
    if (SUPPORTED_IMAGE_API_FORMATS.has(format.api_format)) {
      values.add("image_generation");
    }
  }
  if (values.size === 0) {
    values.add(preset.provider_kind);
  }
  return Array.from(values);
}

function presetSupportsCurrentRuntime(preset: ProviderPreset): boolean {
  return !!getSupportedPresetFormat(preset);
}

export function presetAllowsNonRuntimeConfig(preset: ProviderPreset | null): boolean {
  return !!preset && CONFIGURABLE_NON_RUNTIME_PRESET_KEYS.has(preset.preset_key);
}

export function presetUsesBuiltinEndpoint(preset: ProviderPreset | null): boolean {
  return !!preset && preset.preset_key !== "custom";
}

export function presetIsConfigurable(preset: ProviderPreset): boolean {
  return presetProviderKinds(preset).includes("image_generation")
    || presetSupportsCurrentRuntime(preset)
    || presetAllowsNonRuntimeConfig(preset);
}

function apiFormatSupportedForKind(providerKind: ProviderKind, apiFormat: ProviderApiFormat): boolean {
  if (providerKind === "image_generation") {
    return SUPPORTED_IMAGE_API_FORMATS.has(apiFormat);
  }
  return SUPPORTED_AGENT_API_FORMATS.has(apiFormat);
}

export function buildProviderDraft(
  presets: ProviderPreset[],
  presetKey = "anthropic",
): ProviderDraft {
  const preset = presets.find((item) => item.preset_key === presetKey) ?? presets[0] ?? null;
  const providerKind = preset?.provider_kind ?? "llm";
  const supportedFormat = getSupportedPresetFormat(preset, providerKind);
  const format = supportedFormat ?? getPresetFormat(preset);
  const providerKey = preset ? getPresetProviderKey(preset) : "";
  const isCustom = preset?.preset_key === "custom";
  return {
    provider_kind: providerKind,
    provider: isCustom ? "" : providerKey,
    preset_key: preset?.preset_key ?? "custom",
    api_format: (format?.api_format ?? preset?.default_api_format ?? DEFAULT_AGENT_API_FORMAT) as ProviderApiFormat,
    display_name: isCustom ? "" : (preset?.display_name ?? ""),
    auth_token: "",
    base_url: format?.base_url ?? "",
    models_path: format?.models_path ?? "",
    enabled: false,
  };
}

export function toProviderDraft(item: ProviderConfigRecord): ProviderDraft {
  return {
    provider_kind: item.provider_kind,
    provider: item.provider,
    preset_key: item.preset_key || "custom",
    api_format: item.api_format,
    display_name: item.display_name || item.provider,
    auth_token: "",
    base_url: item.base_url,
    models_path: item.models_path || "",
    enabled: item.enabled,
  };
}

function stringifyJson(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2);
}

export function getProviderTitle(item: ProviderConfigRecord): string {
  return item.display_name || item.provider;
}

export function isCustomProviderRecord(item: ProviderConfigRecord): boolean {
  return !item.preset_key || item.preset_key === "custom";
}

export function getUsageAgentTitle(agent: ProviderConfigRecord["used_by_agents"][number]): string {
  return agent.display_name?.trim() || agent.name?.trim() || agent.agent_id;
}

type TranslateFn = (key: TranslationKey, params?: Record<string, string | number>) => string;

export function parseProviderOptions(raw: string, invalidObjectMessage: string): Record<string, unknown> {
  const trimmed = raw.trim();
  if (!trimmed) {
    return {};
  }
  const parsed = JSON.parse(trimmed);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(invalidObjectMessage);
  }
  return parsed as Record<string, unknown>;
}

function getEffectiveEndpointFormat(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): ProviderPresetFormat | null {
  if (!presetUsesBuiltinEndpoint(preset)) {
    return null;
  }
  return getPresetFormat(preset, draft.api_format);
}

function getEffectiveBaseUrl(draft: ProviderDraft, preset: ProviderPreset | null): string {
  return getEffectiveEndpointFormat(draft, preset)?.base_url ?? draft.base_url;
}

export function getEffectiveModelsPath(draft: ProviderDraft, preset: ProviderPreset | null): string {
  return getEffectiveEndpointFormat(draft, preset)?.models_path ?? draft.models_path;
}

export function buildProviderPayloadFromDraft(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): UpdateProviderConfigPayload {
  return {
    provider_kind: draft.provider_kind,
    preset_key: draft.preset_key,
    api_format: draft.api_format,
    display_name: draft.display_name.trim() || draft.provider.trim(),
    base_url: getEffectiveBaseUrl(draft, preset).trim(),
    models_path: getEffectiveModelsPath(draft, preset).trim(),
    enabled: draft.enabled,
  };
}

export function getProviderDraftError(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  isCreating: boolean,
  translate: TranslateFn,
): string | null {
  if (!draft.provider.trim() && draft.preset_key === "custom") {
    return translate("settings.providers.validation_provider_name_required");
  }
  if (!draft.provider.trim()) {
    return translate("settings.providers.validation_provider_required");
  }
  if (!getEffectiveBaseUrl(draft, preset).trim()) {
    return translate("settings.providers.validation_base_url_required");
  }
  if (!draft.api_format.trim()) {
    return translate("settings.providers.validation_api_format_required");
  }
  if (isCreating && !draft.auth_token.trim()) {
    return translate("settings.providers.validation_auth_token_required");
  }
  return null;
}

export function providerDraftHasChanges(
  draft: ProviderDraft,
  record: ProviderConfigRecord | null,
  preset: ProviderPreset | null,
): boolean {
  if (!record) {
    return true;
  }
  if (draft.auth_token.trim()) {
    return true;
  }
  return draft.preset_key !== (record.preset_key || "custom")
    || draft.api_format !== record.api_format
    || (draft.display_name.trim() || draft.provider.trim()) !== (record.display_name || record.provider)
    || getEffectiveBaseUrl(draft, preset).trim() !== record.base_url
    || getEffectiveModelsPath(draft, preset).trim() !== (record.models_path || "")
    || draft.enabled !== record.enabled;
}

export function formatTokenPreview(maskedToken: string | null | undefined, emptyLabel: string): string {
  const normalizedMaskedToken = maskedToken?.trim();
  if (!normalizedMaskedToken) {
    return emptyLabel;
  }
  return normalizedMaskedToken;
}

export function providerHasActiveConfig(item: ProviderConfigRecord | null | undefined): boolean {
  return !!item?.enabled && !!item.auth_token_masked?.trim();
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

export function normalizeCustomProviderKey(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function getEffectiveCapabilities(model: ProviderModelRecord): ProviderModelCapabilities {
  return {
    ...model.capabilities_auto,
    ...model.capabilities_override,
  };
}

export function sortModelsEnabledFirst(models: ProviderModelRecord[]): ProviderModelRecord[] {
  return [...models].sort((left, right) => {
    if (left.enabled !== right.enabled) {
      return left.enabled ? -1 : 1;
    }
    return (left.display_name || left.model_id).localeCompare(right.display_name || right.model_id, "zh-Hans-CN");
  });
}

export function filterProviderModels(models: ProviderModelRecord[], rawQuery: string): ProviderModelRecord[] {
  const query = rawQuery.trim().toLowerCase();
  if (!query) {
    return models;
  }
  return models.filter((model) => (
    model.model_id.toLowerCase().includes(query)
    || model.display_name.toLowerCase().includes(query)
    || model.category.toLowerCase().includes(query)
  ));
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
        label: displayName === model.model_id ? model.model_id : `${displayName} · ${model.model_id}`,
      };
    }),
  ];
}

export function orderProviderRecords(
  items: ProviderConfigRecord[],
  previousItems: ProviderConfigRecord[],
): ProviderConfigRecord[] {
  const previousIndexMap = new Map(previousItems.map((item, index) => [item.provider, index]));
  return [...items].sort((left, right) => {
    const leftIndex = previousIndexMap.get(left.provider);
    const rightIndex = previousIndexMap.get(right.provider);
    if (leftIndex !== undefined && rightIndex !== undefined) {
      return leftIndex - rightIndex;
    }
    if (leftIndex !== undefined) {
      return -1;
    }
    if (rightIndex !== undefined) {
      return 1;
    }
    return getProviderTitle(left).localeCompare(getProviderTitle(right), "zh-Hans-CN");
  });
}

export function firstBuiltinPresetKey(presets: ProviderPreset[]): string | null {
  return presets.find((preset) => preset.preset_key !== "custom")?.preset_key ?? null;
}

export function providerForPreset(
  items: ProviderConfigRecord[],
  presetKey: string,
): ProviderConfigRecord | null {
  return items.find((item) => item.preset_key === presetKey) ?? null;
}

export function modelOptionsFromRecord(model: ProviderModelRecord): ModelOptionsState {
  return {
    model,
    capabilities: { ...model.capabilities_override },
    context_window: model.context_window ? String(model.context_window) : "",
    max_output_tokens: model.max_output_tokens ? String(model.max_output_tokens) : "",
    provider_options_text: stringifyJson(model.provider_options ?? {}),
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
