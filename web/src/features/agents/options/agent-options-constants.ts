/**
 * AgentOptions Provider 常量与归一化工具
 */

import { format_provider_label, type ProviderOption } from "@/types/capability/provider";

export const DEFAULT_AGENT_OPTION_PROVIDER = "";

export function normalize_agent_option_provider(provider?: string | null): string {
  const normalized_provider = provider?.trim();
  return normalized_provider || DEFAULT_AGENT_OPTION_PROVIDER;
}

export function build_agent_option_provider_options(
  provider_options: ProviderOption[],
  current_provider?: string,
): ProviderOption[] {
  const normalized_provider = current_provider?.trim();
  if (!normalized_provider || provider_options.some((item) => item.provider === normalized_provider)) {
    return provider_options;
  }
  return [
    ...provider_options,
    {
      provider: normalized_provider,
      display_name: format_provider_label(normalized_provider),
      is_default: false,
    },
  ];
}
