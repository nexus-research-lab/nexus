import { useCallback, useEffect, useMemo, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import { invalidateProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { listProviderPresetsApi } from "@/lib/api/provider-config-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import type { ProviderSettingsApi } from "./provider-settings-api";
import {
  firstBuiltinPresetKey,
  orderProviderRecords,
  providerForPreset,
} from "./model/provider-catalog-model";
import { toProviderDraft } from "./model/provider-config-model";
import { buildProviderDraft } from "./model/provider-preset-model";
import {
  type FeedbackState,
  type FormMode,
  type ProviderDraft,
} from "./model/provider-settings-types";

interface ProviderWorkspaceState {
  draft: ProviderDraft;
  mode: FormMode;
  presets: ProviderPreset[];
  providers: ProviderConfigRecord[];
  selectedProvider: string | null;
}

interface UseProviderWorkspaceOptions {
  listConfigs: ProviderSettingsApi["listConfigs"];
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  visibilityScope: ProviderConfigRecord["visibility"];
}

const INITIAL_WORKSPACE: ProviderWorkspaceState = {
  draft: buildProviderDraft([]),
  mode: "empty",
  presets: [],
  providers: [],
  selectedProvider: null,
};

function refreshedWorkspace(
  current: ProviderWorkspaceState,
  presets: ProviderPreset[],
  providers: ProviderConfigRecord[],
  visibility: ProviderConfigRecord["visibility"],
  preferredProvider?: string | null,
): ProviderWorkspaceState {
  const scopedProviders = providers.filter((item) => item.visibility === visibility);
  const orderedProviders = orderProviderRecords(scopedProviders, current.providers);
  const target = orderedProviders.find((item) => item.provider === preferredProvider)
    ?? orderedProviders.find((item) => item.provider === current.selectedProvider);
  if (target) {
    return {
      draft: toProviderDraft(target),
      mode: "edit",
      presets,
      providers: orderedProviders,
      selectedProvider: target.provider,
    };
  }

  const firstPresetKey = firstBuiltinPresetKey(presets);
  const presetTarget = firstPresetKey
    ? providerForPreset(orderedProviders, firstPresetKey)
    : null;
  if (presetTarget) {
    return {
      draft: toProviderDraft(presetTarget),
      mode: "edit",
      presets,
      providers: orderedProviders,
      selectedProvider: presetTarget.provider,
    };
  }
  return {
    draft: buildProviderDraft(presets, firstPresetKey ?? "custom"),
    mode: "create",
    presets,
    providers: orderedProviders,
    selectedProvider: null,
  };
}

export function useProviderWorkspace({
  listConfigs,
  setFeedback,
  t,
  visibilityScope,
}: UseProviderWorkspaceOptions) {
  const [workspace, setWorkspace] = useState(INITIAL_WORKSPACE);
  const [loading, setLoading] = useState(true);
  const { draft, mode, presets, providers, selectedProvider } = workspace;
  const selectedRecord = useMemo(
    () => providers.find((item) => item.provider === selectedProvider) ?? null,
    [providers, selectedProvider],
  );
  const currentPreset = useMemo(
    () => presets.find((item) => item.preset_key === draft.preset_key)
      ?? presets.find((item) => item.preset_key === "custom")
      ?? null,
    [draft.preset_key, presets],
  );

  const refreshAll = useCallback(async (preferredProvider?: string | null) => {
    try {
      const [nextPresets, nextProviders] = await Promise.all([
        listProviderPresetsApi(),
        listConfigs(),
      ]);
      setWorkspace((current) => refreshedWorkspace(
        current,
        nextPresets,
        nextProviders,
        visibilityScope,
        preferredProvider,
      ));
      invalidateProviderAvailability();
      setFeedback((current) => current?.tone === "error" ? null : current);
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.load_failed_title"),
        message: error instanceof Error
          ? error.message
          : t("settings.providers.retry_later"),
      });
    } finally {
      setLoading(false);
    }
  }, [listConfigs, setFeedback, t, visibilityScope]);

  useEffect(() => {
    void refreshAll();
  }, [refreshAll]);

  const updateDraft = useCallback((patch: Partial<ProviderDraft>) => {
    setWorkspace((current) => ({
      ...current,
      draft: { ...current.draft, ...patch },
    }));
  }, []);

  const selectProvider = useCallback((provider: string): boolean => {
    const target = providers.find((item) => item.provider === provider);
    if (!target) {
      return false;
    }
    setWorkspace((current) => ({
      ...current,
      draft: toProviderDraft(target),
      mode: "edit",
      selectedProvider: target.provider,
    }));
    return true;
  }, [providers]);

  const createFromPreset = useCallback((presetKey: string) => {
    setWorkspace((current) => ({
      ...current,
      draft: buildProviderDraft(current.presets, presetKey),
      mode: "create",
      selectedProvider: null,
    }));
  }, []);

  return {
    currentPreset,
    draft,
    loading,
    mode,
    presets,
    providers,
    refreshAll,
    selectedProvider,
    selectedRecord,
    createFromPreset,
    selectProvider,
    updateDraft,
  };
}
