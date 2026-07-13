import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import {
  firstBuiltinPresetKey,
  orderProviderRecords,
  providerForPreset,
} from "../model/provider-catalog-model";
import { toProviderDraft } from "../model/provider-config-model";
import { buildProviderDraft } from "../model/provider-preset-model";
import type {
  FormMode,
  ProviderDraft,
} from "../model/provider-settings-types";

export interface ProviderWorkspaceState {
  draft: ProviderDraft;
  mode: FormMode;
  presets: ProviderPreset[];
  providers: ProviderConfigRecord[];
  selectedProvider: string | null;
}

export const INITIAL_PROVIDER_WORKSPACE: ProviderWorkspaceState = {
  draft: buildProviderDraft([]),
  mode: "empty",
  presets: [],
  providers: [],
  selectedProvider: null,
};

function findProviderByPriority(
  providers: ProviderConfigRecord[],
  providerKeys: Array<string | null | undefined>,
): ProviderConfigRecord | null {
  for (const key of providerKeys) {
    const target = providers.find((item) => item.provider === key);
    if (target) {
      return target;
    }
  }
  return null;
}

function editWorkspace(
  presets: ProviderPreset[],
  providers: ProviderConfigRecord[],
  target: ProviderConfigRecord,
): ProviderWorkspaceState {
  return {
    draft: toProviderDraft(target),
    mode: "edit",
    presets,
    providers,
    selectedProvider: target.provider,
  };
}

export function refreshProviderWorkspace(
  current: ProviderWorkspaceState,
  presets: ProviderPreset[],
  providers: ProviderConfigRecord[],
  visibility: ProviderConfigRecord["visibility"],
  preferredProvider?: string | null,
): ProviderWorkspaceState {
  const scopedProviders = providers.filter(
    (item) => item.visibility === visibility,
  );
  const orderedProviders = orderProviderRecords(scopedProviders, current.providers);
  const selectedTarget = findProviderByPriority(orderedProviders, [
    preferredProvider,
    current.selectedProvider,
  ]);
  if (selectedTarget) {
    return editWorkspace(presets, orderedProviders, selectedTarget);
  }

  const firstPresetKey = firstBuiltinPresetKey(presets);
  const presetTarget = firstPresetKey
    ? providerForPreset(orderedProviders, firstPresetKey)
    : null;
  if (presetTarget) {
    return editWorkspace(presets, orderedProviders, presetTarget);
  }
  return {
    draft: buildProviderDraft(presets, firstPresetKey ?? "custom"),
    mode: "create",
    presets,
    providers: orderedProviders,
    selectedProvider: null,
  };
}

export function updateProviderWorkspaceDraft(
  current: ProviderWorkspaceState,
  patch: Partial<ProviderDraft>,
): ProviderWorkspaceState {
  return {
    ...current,
    draft: { ...current.draft, ...patch },
  };
}

export function selectProviderWorkspace(
  current: ProviderWorkspaceState,
  target: ProviderConfigRecord,
): ProviderWorkspaceState {
  return editWorkspace(current.presets, current.providers, target);
}

export function createProviderWorkspace(
  current: ProviderWorkspaceState,
  presetKey: string,
): ProviderWorkspaceState {
  return {
    ...current,
    draft: buildProviderDraft(current.presets, presetKey),
    mode: "create",
    selectedProvider: null,
  };
}
