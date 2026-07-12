import type { AgentOptions } from "@/types/agent/agent";
import type { ModelSelectionPreference } from "@/types/settings/preferences";

export interface NormalizedModelSelection {
  model: string;
  provider: string;
}

function copyPreferredList<T>(
  preferred: readonly T[] | undefined,
  fallback: readonly T[] | undefined,
  defaultValue: readonly T[],
): T[] {
  return [...(preferred ?? fallback ?? defaultValue)];
}

export function mergeAgentOptions(
  fallback: Partial<AgentOptions>,
  preferred?: Partial<AgentOptions> | null,
): Partial<AgentOptions> {
  const source = preferred ?? {};
  return {
    ...fallback,
    ...source,
    allowed_tools: copyPreferredList(
      source.allowed_tools,
      fallback.allowed_tools,
      [],
    ),
    disallowed_tools: copyPreferredList(
      source.disallowed_tools,
      fallback.disallowed_tools,
      [],
    ),
    setting_sources: copyPreferredList(
      source.setting_sources,
      fallback.setting_sources,
      ["project"],
    ),
  };
}

export function normalizeModelSelectionPreference(
  selection?: ModelSelectionPreference | null,
): NormalizedModelSelection | undefined {
  const provider = selection?.provider?.trim();
  const model = selection?.model?.trim();
  if (!provider || !model) {
    return undefined;
  }
  return { provider, model };
}
