import type {
  ProviderApiFormat,
  ProviderKind,
  ProviderPreset,
  ProviderPresetFormat,
} from "@/types/capability/provider";

import type { ProviderDraft } from "./provider-settings-types";

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
const PROVIDER_KIND_ORDER: ProviderKind[] = ["llm", "image_generation"];

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

function apiFormatSupportedForKind(
  providerKind: ProviderKind,
  apiFormat: ProviderApiFormat,
): boolean {
  return providerKind === "image_generation"
    ? SUPPORTED_IMAGE_API_FORMATS.has(apiFormat)
    : SUPPORTED_AGENT_API_FORMATS.has(apiFormat);
}

export function getPresetFormat(
  preset: ProviderPreset | null,
  apiFormat?: ProviderApiFormat,
): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const targetFormat = apiFormat ?? preset.default_api_format;
  return preset.formats.find((item) => item.api_format === targetFormat)
    ?? preset.formats[0]
    ?? null;
}

export function formatSupportsProviderKind(
  format: ProviderPresetFormat,
  providerKind: ProviderKind,
): boolean {
  return format.provider_kind
    ? format.provider_kind === providerKind
    : apiFormatSupportedForKind(providerKind, format.api_format);
}

export function getSupportedPresetFormat(
  preset: ProviderPreset | null,
  providerKind?: ProviderKind,
): ProviderPresetFormat | null {
  if (!preset) {
    return null;
  }
  const targetKind = providerKind ?? preset.provider_kind;
  return preset.formats.find((item) => item.provider_kind === targetKind)
    ?? preset.formats.find((item) => (
      formatSupportsProviderKind(item, targetKind)
    ))
    ?? null;
}

export function presetProviderKinds(
  preset: ProviderPreset | null,
): ProviderKind[] {
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

export function orderedPresetProviderKinds(
  preset: ProviderPreset | null,
): ProviderKind[] {
  const availableKinds = presetProviderKinds(preset);
  return PROVIDER_KIND_ORDER.filter((kind) => (
    availableKinds.length === 0 || availableKinds.includes(kind)
  ));
}

export function uniquePresetFormats(
  preset: ProviderPreset | null,
): ProviderPresetFormat[] {
  const seen = new Set<ProviderApiFormat>();
  return (preset?.formats ?? []).filter((format) => {
    if (seen.has(format.api_format)) {
      return false;
    }
    seen.add(format.api_format);
    return true;
  });
}

export function presetAllowsNonRuntimeConfig(
  preset: ProviderPreset | null,
): boolean {
  return !!preset && CONFIGURABLE_NON_RUNTIME_PRESET_KEYS.has(preset.preset_key);
}

export function presetUsesBuiltinEndpoint(
  preset: ProviderPreset | null,
): boolean {
  return !!preset && preset.preset_key !== "custom";
}

export function presetIsConfigurable(preset: ProviderPreset): boolean {
  return presetProviderKinds(preset).includes("image_generation")
    || !!getSupportedPresetFormat(preset)
    || presetAllowsNonRuntimeConfig(preset);
}

export function buildProviderDraft(
  presets: ProviderPreset[],
  presetKey = "anthropic",
): ProviderDraft {
  const preset = presets.find((item) => item.preset_key === presetKey)
    ?? presets[0]
    ?? null;
  const providerKind = preset?.provider_kind ?? "llm";
  const format = getSupportedPresetFormat(preset, providerKind)
    ?? getPresetFormat(preset);
  const isCustom = preset?.preset_key === "custom";
  return {
    provider_kind: providerKind,
    provider: isCustom ? "" : PRESET_PROVIDER_KEYS[preset?.preset_key ?? ""] ?? "",
    preset_key: preset?.preset_key ?? "custom",
    api_format: format?.api_format
      ?? preset?.default_api_format
      ?? DEFAULT_AGENT_API_FORMAT,
    display_name: isCustom ? "" : preset?.display_name ?? "",
    auth_token: "",
    base_url: format?.base_url ?? "",
    models_path: format?.models_path ?? "",
    enabled: false,
  };
}
