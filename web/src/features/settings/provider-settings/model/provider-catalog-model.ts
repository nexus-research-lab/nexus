import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import { getProviderTitle } from "./provider-config-model";

export interface ProviderCatalog {
  configuredByPreset: Map<string, ProviderConfigRecord>;
  customProviders: ProviderConfigRecord[];
}

export function buildProviderCatalog(
  providers: ProviderConfigRecord[],
): ProviderCatalog {
  const configuredByPreset = new Map<string, ProviderConfigRecord>();
  for (const provider of providers) {
    if (
      provider.preset_key
      && provider.preset_key !== "custom"
      && !configuredByPreset.has(provider.preset_key)
    ) {
      configuredByPreset.set(provider.preset_key, provider);
    }
  }
  return {
    configuredByPreset,
    customProviders: providers.filter((provider) => (
      provider.preset_key === "custom"
      || !configuredByPreset.has(provider.preset_key)
    )),
  };
}

export function orderProviderRecords(
  items: ProviderConfigRecord[],
  previousItems: ProviderConfigRecord[],
): ProviderConfigRecord[] {
  const previousIndexes = new Map(
    previousItems.map((item, index) => [item.provider, index]),
  );
  return [...items].sort((left, right) => {
    const leftIndex = previousIndexes.get(left.provider);
    const rightIndex = previousIndexes.get(right.provider);
    if (leftIndex !== undefined && rightIndex !== undefined) {
      return leftIndex - rightIndex;
    }
    if (leftIndex !== undefined) {
      return -1;
    }
    if (rightIndex !== undefined) {
      return 1;
    }
    return getProviderTitle(left).localeCompare(
      getProviderTitle(right),
      "zh-Hans-CN",
    );
  });
}

export function firstBuiltinPresetKey(
  presets: ProviderPreset[],
): string | null {
  return presets.find((preset) => preset.preset_key !== "custom")?.preset_key
    ?? null;
}

export function providerForPreset(
  items: ProviderConfigRecord[],
  presetKey: string,
): ProviderConfigRecord | null {
  return items.find((item) => item.preset_key === presetKey) ?? null;
}
