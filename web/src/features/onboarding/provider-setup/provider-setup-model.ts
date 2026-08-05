import type {
  ProviderConfigRecord,
  ProviderPreset,
  ProviderPresetFormat,
} from "@/types/capability/provider";
import type { AgentRuntimeKind } from "@/types/settings/preferences";

const AGENT_API_FORMATS = new Set([
  "anthropic_messages",
  "chat_completions",
  "responses",
]);

// 初始化时优先展示国内可直接获取 API Key 的服务；未列入的预设保持服务端目录顺序。
const DOMESTIC_PROVIDER_ORDER = [
  "deepseek",
  "qwen-token-plan",
  "minimax-token-plan",
  "glm-coding-plan",
  "kimi-code",
  "volcengine-coding-plan",
  "doubao",
  "dashscope",
  "modelscope",
] as const;
const DOMESTIC_PROVIDER_RANK = new Map<string, number>(
  DOMESTIC_PROVIDER_ORDER.map((presetKey, index) => [presetKey, index]),
);

export interface ProviderSetupPreset {
  format: ProviderPresetFormat;
  preset: ProviderPreset;
}

/**
 * 中文注释：初始化向导只投影能直接作为当前 Agent 对话模型的预设。
 * 图片 Provider、未知协议和 custom 入口留给完整 Settings 页面。
 */
export function listProviderSetupPresets(
  presets: readonly ProviderPreset[],
  runtimeKind: AgentRuntimeKind,
  providers: readonly ProviderConfigRecord[] = [],
): ProviderSetupPreset[] {
  return presets
    .filter((preset) => preset.preset_key !== "custom")
    .map((preset) => {
      const configured = findManageablePresetProvider(
        providers,
        preset.preset_key,
      );
      const format = resolveProviderSetupFormat(
        preset,
        runtimeKind,
        configured?.api_format,
      );
      return format ? { format, preset } : null;
    })
    .filter((item): item is ProviderSetupPreset => item !== null)
    .sort(compareProviderSetupPresets);
}

function compareProviderSetupPresets(
  left: ProviderSetupPreset,
  right: ProviderSetupPreset,
): number {
  const leftRank = DOMESTIC_PROVIDER_RANK.get(left.preset.preset_key);
  const rightRank = DOMESTIC_PROVIDER_RANK.get(right.preset.preset_key);
  if (leftRank === undefined && rightRank === undefined) {
    return 0;
  }
  if (leftRank === undefined) {
    return 1;
  }
  if (rightRank === undefined) {
    return -1;
  }
  return leftRank - rightRank;
}

export function resolveProviderSetupFormat(
  preset: ProviderPreset,
  runtimeKind: AgentRuntimeKind,
  preferredAPIFormat?: ProviderPresetFormat["api_format"],
): ProviderPresetFormat | null {
  const compatibleFormats = preset.formats.filter((format) => (
    isLLMFormat(format)
    && isRuntimeCompatibleFormat(format.api_format, runtimeKind)
  ));
  const configuredFormat = compatibleFormats.find(
    (format) => format.api_format === preferredAPIFormat,
  );
  if (configuredFormat) {
    return configuredFormat;
  }
  return compatibleFormats.find(
    (format) => format.api_format === preset.default_api_format,
  ) ?? compatibleFormats[0] ?? null;
}

export function findManageablePresetProvider(
  providers: readonly ProviderConfigRecord[],
  presetKey: string,
): ProviderConfigRecord | null {
  return providers.find((provider) => (
    provider.can_manage
    && provider.preset_key === presetKey
  )) ?? null;
}

export function selectInitialProviderSetupPreset(
  presets: readonly ProviderSetupPreset[],
  providers: readonly ProviderConfigRecord[],
): ProviderSetupPreset | null {
  let firstConfigured: ProviderSetupPreset | null = null;
  for (const preset of presets) {
    const provider = findManageablePresetProvider(
      providers,
      preset.preset.preset_key,
    );
    if (!provider) {
      continue;
    }
    if (provider.enabled) {
      return preset;
    }
    firstConfigured ??= preset;
  }
  return firstConfigured ?? presets[0] ?? null;
}

export function providerSetupModelIsRequired(
  format: ProviderPresetFormat,
  existingProvider: ProviderConfigRecord | null,
): boolean {
  if (format.models_path.trim()) {
    return false;
  }
  return !(existingProvider?.models ?? []).some((model) => model.enabled);
}

function isLLMFormat(format: ProviderPresetFormat): boolean {
  if (format.provider_kind) {
    return format.provider_kind === "llm";
  }
  return AGENT_API_FORMATS.has(format.api_format);
}

function isRuntimeCompatibleFormat(
  apiFormat: ProviderPresetFormat["api_format"],
  runtimeKind: AgentRuntimeKind,
): boolean {
  return apiFormat === "anthropic_messages"
    || runtimeKind === "nxs"
      && (apiFormat === "chat_completions" || apiFormat === "responses");
}
