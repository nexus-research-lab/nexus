import { useCallback, useEffect, useRef, useState } from "react";

import { getDesktopRuntimeConfig } from "@/config/desktop-runtime/runtime-config";
import {
  getDesktopStateRoot,
  relocateDesktopStateRoot,
} from "@/lib/desktop-bridge";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  EMPTY_WORKSPACE_SETTINGS_SNAPSHOT,
  buildStateRootSettingsSnapshot,
  canSaveWorkspaceSettings,
  getStateRootPlaceholderKey,
  replaceWorkspaceDraft,
} from "./model/workspace-settings-model";

export function useWorkspaceSettings() {
  const { t } = useI18n();
  const runtime = getDesktopRuntimeConfig();
  const placeholder = t(getStateRootPlaceholderKey(runtime?.platform));
  const [snapshot, setSnapshot] = useState(
    EMPTY_WORKSPACE_SETTINGS_SNAPSHOT,
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [feedbackMessage, setFeedbackMessage] = useState("");
  const savingRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void getDesktopStateRoot()
      .then((result) => {
        if (!cancelled) {
          setSnapshot(buildStateRootSettingsSnapshot(result));
          setFeedbackMessage(result.migration_error ?? "");
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setFeedbackMessage(getErrorMessage(
            error,
            t("settings.general.state_root_load_failed"),
          ));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [t]);

  const save = useCallback(async () => {
    if (savingRef.current) {
      return;
    }
    savingRef.current = true;
    setSaving(true);
    setFeedbackMessage("");
    try {
      await relocateDesktopStateRoot(snapshot.draftPath.trim());
      setFeedbackMessage(t("settings.general.state_root_restarting"));
    } catch (error) {
      setFeedbackMessage(getErrorMessage(
        error,
        t("settings.general.state_root_save_failed"),
      ));
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }, [snapshot.draftPath, t]);

  const busy = loading || saving;
  return {
    busy,
    currentPath: snapshot.currentPath,
    draftPath: snapshot.draftPath,
    feedbackMessage,
    placeholder,
    save,
    saveDisabled: !canSaveWorkspaceSettings(snapshot, busy),
    saving,
    setDraftPath: (value: string) => {
      setSnapshot((current) => replaceWorkspaceDraft(current, value));
    },
  };
}
