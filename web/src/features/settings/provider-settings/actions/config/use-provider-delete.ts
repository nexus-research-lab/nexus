import { useCallback, useMemo, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import type { ProviderSettingsApi } from "../../provider-settings-api";
import {
  getProviderTitle,
  isCustomProviderRecord,
} from "../../model/provider-config-model";
import {
  buildProviderCommittedRefreshFeedback,
  buildProviderErrorFeedback,
} from "../../model/provider-feedback-model";
import type { FeedbackState } from "../../model/provider-settings-types";
import type { RunProviderCommand } from "../use-provider-command";

type DeleteDialogState = {
  kind: "confirm" | "usage";
  provider: string;
} | null;

interface UseProviderDeleteOptions {
  providerApi: ProviderSettingsApi;
  providers: ProviderConfigRecord[];
  refreshAll: (preferredProvider?: string | null) => Promise<boolean>;
  runCommand: RunProviderCommand;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
}

export function useProviderDelete({
  providerApi,
  providers,
  refreshAll,
  runCommand,
  setFeedback,
  t,
}: UseProviderDeleteOptions) {
  const [dialog, setDialog] = useState<DeleteDialogState>(null);
  const targetRecord = useMemo(
    () => providers.find((item) => item.provider === dialog?.provider) ?? null,
    [dialog?.provider, providers],
  );

  const requestDelete = useCallback((item: ProviderConfigRecord) => {
    if (!isCustomProviderRecord(item)) {
      return;
    }
    setDialog({
      kind: item.usage_count > 0 ? "usage" : "confirm",
      provider: item.provider,
    });
  }, []);

  const deleteProvider = useCallback((force = false) => {
    if (!targetRecord) {
      return;
    }
    if (targetRecord.usage_count > 0 && !force) {
      setDialog({ kind: "usage", provider: targetRecord.provider });
      return;
    }
    void runCommand({ kind: "delete-provider" }, async () => {
      try {
        const result = await providerApi.deleteConfig(targetRecord.provider, {
          force,
        });
        setDialog(null);
        if (!await refreshAll()) {
          setFeedback(buildProviderCommittedRefreshFeedback(
            t("settings.providers.refresh_after_change_failed_message"),
            t,
          ));
          return;
        }
        setFeedback({
          tone: "success",
          title: t("settings.providers.deleted_title"),
          message: result.fallback_to_default
            ? t("settings.providers.delete_follow_default_message", {
              count: result.affected_runtime_count ?? 0,
            })
            : t("settings.providers.delete_removed_message", {
              name: getProviderTitle(targetRecord),
            }),
        });
      } catch (error) {
        setDialog(null);
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.delete_failed_title"),
          t("settings.providers.delete_in_use_fallback"),
          t,
        ));
      }
    });
  }, [
    providerApi,
    refreshAll,
    runCommand,
    setFeedback,
    t,
    targetRecord,
  ]);

  return {
    closeDeleteDialog: () => setDialog(null),
    deleteConfirmOpen: dialog?.kind === "confirm",
    deleteTargetRecord: targetRecord,
    deleteUsageOpen: dialog?.kind === "usage",
    handleDelete: deleteProvider,
    handleRequestDeleteProvider: requestDelete,
  };
}
