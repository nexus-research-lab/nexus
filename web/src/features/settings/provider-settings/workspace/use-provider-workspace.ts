import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import { invalidateProviderAvailability } from "@/hooks/capability/use-provider-availability";
import { listProviderPresetsApi } from "@/lib/api/provider-config-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import type { ProviderSettingsApi } from "../provider-settings-api";
import type {
  FeedbackState,
  ProviderDraft,
} from "../model/provider-settings-types";
import {
  createProviderWorkspace,
  INITIAL_PROVIDER_WORKSPACE,
  refreshProviderWorkspace,
  selectProviderWorkspace,
  updateProviderWorkspaceDraft,
} from "./provider-workspace-model";

interface UseProviderWorkspaceOptions {
  listConfigs: ProviderSettingsApi["listConfigs"];
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  visibilityScope: ProviderConfigRecord["visibility"];
}

export function useProviderWorkspace({
  listConfigs,
  setFeedback,
  t,
  visibilityScope,
}: UseProviderWorkspaceOptions) {
  const [workspace, setWorkspace] = useState(INITIAL_PROVIDER_WORKSPACE);
  const [loading, setLoading] = useState(true);
  const requestIdRef = useRef(0);
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
    const requestId = ++requestIdRef.current;
    try {
      const [nextPresets, nextProviders] = await Promise.all([
        listProviderPresetsApi(),
        listConfigs(),
      ]);
      if (requestId !== requestIdRef.current) {
        return;
      }
      setWorkspace((current) => refreshProviderWorkspace(
        current,
        nextPresets,
        nextProviders,
        visibilityScope,
        preferredProvider,
      ));
      invalidateProviderAvailability();
      setFeedback((current) => current?.tone === "error" ? null : current);
    } catch (error) {
      if (requestId !== requestIdRef.current) {
        return;
      }
      setFeedback({
        tone: "error",
        title: t("settings.providers.load_failed_title"),
        message: error instanceof Error
          ? error.message
          : t("settings.providers.retry_later"),
      });
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [listConfigs, setFeedback, t, visibilityScope]);

  useEffect(() => {
    void refreshAll();
    return () => {
      requestIdRef.current += 1;
    };
  }, [refreshAll]);

  const updateDraft = useCallback((patch: Partial<ProviderDraft>) => {
    setWorkspace((current) => updateProviderWorkspaceDraft(current, patch));
  }, []);

  const selectProvider = useCallback((provider: string): boolean => {
    const target = providers.find((item) => item.provider === provider);
    if (!target) {
      return false;
    }
    setWorkspace((current) => selectProviderWorkspace(current, target));
    return true;
  }, [providers]);

  const createFromPreset = useCallback((presetKey: string) => {
    setWorkspace((current) => createProviderWorkspace(current, presetKey));
  }, []);

  return {
    createFromPreset,
    currentPreset,
    draft,
    loading,
    mode,
    presets,
    providers,
    refreshAll,
    selectProvider,
    selectedProvider,
    selectedRecord,
    updateDraft,
  };
}
