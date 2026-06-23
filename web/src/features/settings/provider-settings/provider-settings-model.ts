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
export type FeedbackTone = "success" | "error";
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

export const SETTINGS_TABS: { key: SettingsTabKey; label_key: "settings.tabs.providers" }[] = [
  { key: "providers", label_key: "settings.tabs.providers" },
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

function get_preset_provider_key(preset: ProviderPreset): string {
  return PRESET_PROVIDER_KEYS[preset.preset_key] ?? "";
}

export function get_preset_format(preset: ProviderPreset | null, api_format?: ProviderApiFormat): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const target_format = api_format ?? preset.default_api_format;
  return preset.formats.find((item) => item.api_format === target_format) ?? preset.formats[0] ?? null;
}

export function format_supports_provider_kind(format: ProviderPresetFormat, provider_kind: ProviderKind): boolean {
  if (format.provider_kind) {
    return format.provider_kind === provider_kind;
  }
  return api_format_supported_for_kind(provider_kind, format.api_format);
}

export function get_supported_preset_format(preset: ProviderPreset | null, provider_kind?: ProviderKind): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const target_kind = provider_kind ?? preset.provider_kind;
  const explicit_format = preset.formats.find((item) => item.provider_kind === target_kind);
  if (explicit_format) {
    return explicit_format;
  }
  return preset.formats.find((item) => format_supports_provider_kind(item, target_kind)) ?? null;
}

export function preset_provider_kinds(preset: ProviderPreset | null): ProviderKind[] {
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

function preset_supports_current_runtime(preset: ProviderPreset): boolean {
  return !!get_supported_preset_format(preset);
}

export function preset_allows_non_runtime_config(preset: ProviderPreset | null): boolean {
  return !!preset && CONFIGURABLE_NON_RUNTIME_PRESET_KEYS.has(preset.preset_key);
}

export function preset_uses_builtin_endpoint(preset: ProviderPreset | null): boolean {
  return !!preset && preset.preset_key !== "custom";
}

export function preset_is_configurable(preset: ProviderPreset): boolean {
  return preset_provider_kinds(preset).includes("image_generation")
    || preset_supports_current_runtime(preset)
    || preset_allows_non_runtime_config(preset);
}

function api_format_supported_for_kind(provider_kind: ProviderKind, api_format: ProviderApiFormat): boolean {
  if (provider_kind === "image_generation") {
    return SUPPORTED_IMAGE_API_FORMATS.has(api_format);
  }
  return SUPPORTED_AGENT_API_FORMATS.has(api_format);
}

export function build_provider_draft(
  presets: ProviderPreset[],
  preset_key = "anthropic",
): ProviderDraft {
  const preset = presets.find((item) => item.preset_key === preset_key) ?? presets[0] ?? null;
  const provider_kind = preset?.provider_kind ?? "llm";
  const supported_format = get_supported_preset_format(preset, provider_kind);
  const format = supported_format ?? get_preset_format(preset);
  const provider_key = preset ? get_preset_provider_key(preset) : "";
  const is_custom = preset?.preset_key === "custom";
  return {
    provider_kind,
    provider: is_custom ? "" : provider_key,
    preset_key: preset?.preset_key ?? "custom",
    api_format: (format?.api_format ?? preset?.default_api_format ?? DEFAULT_AGENT_API_FORMAT) as ProviderApiFormat,
    display_name: is_custom ? "" : (preset?.display_name ?? ""),
    auth_token: "",
    base_url: format?.base_url ?? "",
    models_path: format?.models_path ?? "",
    enabled: false,
  };
}

export function to_provider_draft(item: ProviderConfigRecord): ProviderDraft {
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

function stringify_json(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2);
}

export function get_provider_title(item: ProviderConfigRecord): string {
  return item.display_name || item.provider;
}

export function is_custom_provider_record(item: ProviderConfigRecord): boolean {
  return !item.preset_key || item.preset_key === "custom";
}

export function get_usage_agent_title(agent: ProviderConfigRecord["used_by_agents"][number]): string {
  return agent.display_name?.trim() || agent.name?.trim() || agent.agent_id;
}

type TranslateFn = (key: TranslationKey, params?: Record<string, string | number>) => string;

export function parse_provider_options(raw: string, invalid_object_message: string): Record<string, unknown> {
  const trimmed = raw.trim();
  if (!trimmed) {
    return {};
  }
  const parsed = JSON.parse(trimmed);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(invalid_object_message);
  }
  return parsed as Record<string, unknown>;
}

function get_effective_endpoint_format(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): ProviderPresetFormat | null {
  if (!preset_uses_builtin_endpoint(preset)) {
    return null;
  }
  return get_preset_format(preset, draft.api_format);
}

function get_effective_base_url(draft: ProviderDraft, preset: ProviderPreset | null): string {
  return get_effective_endpoint_format(draft, preset)?.base_url ?? draft.base_url;
}

export function get_effective_models_path(draft: ProviderDraft, preset: ProviderPreset | null): string {
  return get_effective_endpoint_format(draft, preset)?.models_path ?? draft.models_path;
}

export function build_provider_payload_from_draft(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
): UpdateProviderConfigPayload {
  return {
    provider_kind: draft.provider_kind,
    preset_key: draft.preset_key,
    api_format: draft.api_format,
    display_name: draft.display_name.trim() || draft.provider.trim(),
    base_url: get_effective_base_url(draft, preset).trim(),
    models_path: get_effective_models_path(draft, preset).trim(),
    enabled: draft.enabled,
  };
}

export function get_provider_draft_error(
  draft: ProviderDraft,
  preset: ProviderPreset | null,
  is_creating: boolean,
  translate: TranslateFn,
): string | null {
  if (!draft.provider.trim() && draft.preset_key === "custom") {
    return translate("settings.providers.validation_provider_name_required");
  }
  if (!draft.provider.trim()) {
    return translate("settings.providers.validation_provider_required");
  }
  if (!get_effective_base_url(draft, preset).trim()) {
    return translate("settings.providers.validation_base_url_required");
  }
  if (!draft.api_format.trim()) {
    return translate("settings.providers.validation_api_format_required");
  }
  if (is_creating && !draft.auth_token.trim()) {
    return translate("settings.providers.validation_auth_token_required");
  }
  return null;
}

export function provider_draft_has_changes(
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
    || get_effective_base_url(draft, preset).trim() !== record.base_url
    || get_effective_models_path(draft, preset).trim() !== (record.models_path || "")
    || draft.enabled !== record.enabled;
}

export function format_token_preview(masked_token: string | null | undefined, empty_label: string): string {
  const normalized_masked_token = masked_token?.trim();
  if (!normalized_masked_token) {
    return empty_label;
  }
  return normalized_masked_token;
}

export function provider_has_active_config(item: ProviderConfigRecord | null | undefined): boolean {
  return !!item?.enabled && !!item.auth_token_masked?.trim();
}

export function format_count(value?: number | null): string {
  if (!value || value <= 0) {
    return "auto";
  }
  if (value >= 1000) {
    return `${Math.round(value / 1000)}K`;
  }
  return String(value);
}

export function normalize_custom_provider_key(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function get_effective_capabilities(model: ProviderModelRecord): ProviderModelCapabilities {
  return {
    ...model.capabilities_auto,
    ...model.capabilities_override,
  };
}

export function sort_models_enabled_first(models: ProviderModelRecord[]): ProviderModelRecord[] {
  return [...models].sort((left, right) => {
    if (left.enabled !== right.enabled) {
      return left.enabled ? -1 : 1;
    }
    return (left.display_name || left.model_id).localeCompare(right.display_name || right.model_id, "zh-Hans-CN");
  });
}

export function filter_provider_models(models: ProviderModelRecord[], raw_query: string): ProviderModelRecord[] {
  const query = raw_query.trim().toLowerCase();
  if (!query) {
    return models;
  }
  return models.filter((model) => (
    model.model_id.toLowerCase().includes(query)
    || model.display_name.toLowerCase().includes(query)
    || model.category.toLowerCase().includes(query)
  ));
}

export function build_test_model_options(
  models: ProviderModelRecord[],
  auto_label: string,
): { value: string; label: string }[] {
  return [
    { value: AUTO_TEST_MODEL_VALUE, label: auto_label },
    ...sort_models_enabled_first(models).map((model) => {
      const display_name = model.display_name || model.model_id;
      return {
        value: model.model_id,
        label: display_name === model.model_id ? model.model_id : `${display_name} · ${model.model_id}`,
      };
    }),
  ];
}

export function order_provider_records(
  items: ProviderConfigRecord[],
  previous_items: ProviderConfigRecord[],
): ProviderConfigRecord[] {
  const previous_index_map = new Map(previous_items.map((item, index) => [item.provider, index]));
  return [...items].sort((left, right) => {
    const left_index = previous_index_map.get(left.provider);
    const right_index = previous_index_map.get(right.provider);
    if (left_index !== undefined && right_index !== undefined) {
      return left_index - right_index;
    }
    if (left_index !== undefined) {
      return -1;
    }
    if (right_index !== undefined) {
      return 1;
    }
    return get_provider_title(left).localeCompare(get_provider_title(right), "zh-Hans-CN");
  });
}

export function first_builtin_preset_key(presets: ProviderPreset[]): string | null {
  return presets.find((preset) => preset.preset_key !== "custom")?.preset_key ?? null;
}

export function provider_for_preset(
  items: ProviderConfigRecord[],
  preset_key: string,
): ProviderConfigRecord | null {
  return items.find((item) => item.preset_key === preset_key) ?? null;
}

export function model_options_from_record(model: ProviderModelRecord): ModelOptionsState {
  return {
    model,
    capabilities: { ...model.capabilities_override },
    context_window: model.context_window ? String(model.context_window) : "",
    max_output_tokens: model.max_output_tokens ? String(model.max_output_tokens) : "",
    provider_options_text: stringify_json(model.provider_options ?? {}),
  };
}

export function model_update_payload(
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
