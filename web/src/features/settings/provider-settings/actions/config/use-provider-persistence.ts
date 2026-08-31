import { useCallback, useMemo } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import type { ProviderSettingsApi } from "../../provider-settings-api";
import {
  buildProviderCreatePayload,
  buildProviderEnabledPayload,
  buildProviderUpdatePayload,
  getProviderDraftError,
} from "../../model/provider-config-model";
import {
  buildProviderCommittedRefreshFeedback,
  buildProviderErrorFeedback,
  buildProviderValidationFeedback,
} from "../../model/provider-feedback-model";
import type {
  FeedbackState,
  ProviderDraft,
} from "../../model/provider-settings-types";
import type { RunProviderCommand } from "../use-provider-command";
import {
  type ProviderPersistResult,
  type ProviderPersistenceSnapshot,
  type ProviderPersistenceStop,
  resolveProviderPersistenceStop,
} from "./provider-persistence-plan";

interface ProviderPersistenceApi {
  createConfig: ProviderSettingsApi["createConfig"];
  updateConfig: ProviderSettingsApi["updateConfig"];
}

export type PersistProvider = (options?: {
  enabled?: boolean;
  showError?: boolean;
}) => Promise<ProviderPersistResult | null>;

interface UseProviderPersistenceOptions {
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  isCreating: boolean;
  isEditing: boolean;
  isEmptyMode: boolean;
  providerApi: ProviderPersistenceApi;
  refreshAll: (preferredProvider?: string | null) => Promise<boolean>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  updateDraft: (patch: Partial<ProviderDraft>) => void;
  visibilityScope: ProviderConfigRecord["visibility"];
}

export function useProviderPersistence({
  currentPreset,
  draft,
  isCreating,
  isEditing,
  isEmptyMode,
  providerApi,
  refreshAll,
  runCommand,
  selectedCanManage,
  selectedRecord,
  setFeedback,
  t,
  updateDraft,
  visibilityScope,
}: UseProviderPersistenceOptions) {
  const canSave = useMemo(() => (
    !isEmptyMode
    && selectedCanManage
    && getProviderDraftError(draft, currentPreset, isCreating, t) === null
  ), [currentPreset, draft, isCreating, isEmptyMode, selectedCanManage, t]);

  const persistProvider = useCallback<PersistProvider>(async (options) => {
    const snapshot: ProviderPersistenceSnapshot = {
      currentPreset,
      draft: options?.enabled === undefined
        ? draft
        : { ...draft, enabled: options.enabled },
      isCreating,
      isEditing,
      isEmptyMode,
      selectedRecord,
    };
    const showError = options?.showError ?? true;
    const stop = resolveProviderPersistenceStop(snapshot, t);
    if (stop) {
      return resolvePersistenceStop(stop, showError, setFeedback, t);
    }

    try {
      return await executeProviderPersistence(
        snapshot,
        providerApi,
        visibilityScope,
      );
    } catch (error) {
      reportPersistenceFailure(error, setFeedback, t);
      return null;
    }
  }, [
    currentPreset,
    draft,
    isCreating,
    isEditing,
    isEmptyMode,
    providerApi,
    selectedRecord,
    setFeedback,
    t,
    visibilityScope,
  ]);

  const handleProviderFieldBlur = useCallback(() => {
    if (!canSave) {
      return;
    }
    void runCommand({ kind: "save-provider" }, async () => {
      const result = await persistProvider({ showError: false });
      if (result?.changed) {
        if (!await refreshAll(result.record.provider)) {
          setFeedback(buildProviderCommittedRefreshFeedback(
            t("settings.providers.refresh_after_change_failed_message"),
            t,
          ));
          return;
        }
        reportProviderAvailabilityChange(selectedRecord, result.record, setFeedback, t);
      }
    });
  }, [
    canSave,
    persistProvider,
    refreshAll,
    runCommand,
    selectedRecord,
    setFeedback,
    t,
  ]);

  const handleEnabledChange = useCallback((checked: boolean) => {
    if (!selectedCanManage) {
      return;
    }
    void runCommand({ kind: "toggle-provider" }, async () => {
      if (!selectedRecord) {
        const result = await persistProvider({
          enabled: checked,
          showError: true,
        });
        if (result) {
          if (!await refreshAll(result.record.provider)) {
            setFeedback(buildProviderCommittedRefreshFeedback(
              t("settings.providers.refresh_after_change_failed_message"),
              t,
            ));
          }
        }
        return;
      }
      updateDraft({ enabled: checked });
      try {
        const result = await providerApi.updateConfig(
          selectedRecord.provider,
          buildProviderEnabledPayload(
            selectedRecord,
            checked,
            draft.auth_token,
          ),
        );
        if (!await refreshAll(result.provider)) {
          setFeedback(buildProviderCommittedRefreshFeedback(
            t("settings.providers.refresh_after_change_failed_message"),
            t,
          ));
          return;
        }
        reportProviderAvailabilityChange(selectedRecord, result, setFeedback, t);
      } catch (error) {
        const failure = buildProviderErrorFeedback(
          error,
          t("settings.providers.save_failed_title"),
          t("settings.providers.check_config_retry"),
          t,
        );
        if (failure.mutationEffect === "not_applied") {
          updateDraft({ enabled: !checked });
        }
        setFeedback(failure);
      }
    });
  }, [
    draft.auth_token,
    persistProvider,
    providerApi,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
    updateDraft,
  ]);

  return {
    handleEnabledChange,
    handleProviderFieldBlur,
    persistProvider,
  };
}

function reportProviderAvailabilityChange(
  previous: ProviderConfigRecord | null,
  next: ProviderConfigRecord,
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>,
  t: I18nContextValue["t"],
): void {
  if (!previous || previous.usage_count <= 0 || previous.enabled === next.enabled) {
    return;
  }
  if (!next.enabled) {
    setFeedback({
      tone: "success",
      title: t("settings.providers.disabled_title"),
      message: t("settings.providers.disabled_follow_default_message", {
        count: previous.usage_count,
      }),
    });
    return;
  }
  setFeedback({
    tone: "success",
    title: t("settings.providers.restored_title"),
    message: t("settings.providers.restored_model_message"),
  });
}

function resolvePersistenceStop(
  stop: ProviderPersistenceStop,
  showError: boolean,
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>,
  translate: I18nContextValue["t"],
): ProviderPersistResult | null {
  if (stop.kind === "return") {
    return stop.result;
  }
  if (showError) {
    setFeedback(buildProviderValidationFeedback(
      translate("settings.providers.config_incomplete_title"),
      stop.message,
      translate,
    ));
  }
  return null;
}

async function executeProviderPersistence(
  snapshot: ProviderPersistenceSnapshot,
  providerApi: ProviderPersistenceApi,
  visibility: ProviderConfigRecord["visibility"],
): Promise<ProviderPersistResult> {
  const record = snapshot.isEditing && snapshot.selectedRecord
    ? await providerApi.updateConfig(
      snapshot.selectedRecord.provider,
      buildProviderUpdatePayload(snapshot.draft, snapshot.currentPreset),
    )
    : await providerApi.createConfig(buildProviderCreatePayload(
      snapshot.draft,
      snapshot.currentPreset,
      visibility,
    ));
  return { changed: true, record };
}

function reportPersistenceFailure(
  error: unknown,
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>,
  translate: I18nContextValue["t"],
): void {
  setFeedback(buildProviderErrorFeedback(
    error,
    translate("settings.providers.save_failed_title"),
    translate("settings.providers.check_config_retry"),
    translate,
  ));
}
