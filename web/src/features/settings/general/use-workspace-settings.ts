import { useCallback, useEffect, useRef, useState } from "react";

import {
  getRuntimeSettingsApi,
  updateRuntimeSettingsApi,
} from "@/lib/api/settings/runtime-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  EMPTY_WORKSPACE_SETTINGS_SNAPSHOT,
  buildWorkspaceSettingsSnapshot,
  canSaveWorkspaceSettings,
  replaceWorkspaceDraft,
} from "./model/workspace-settings-model";

export function useWorkspaceSettings() {
  const { t } = useI18n();
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
    void getRuntimeSettingsApi()
      .then((result) => {
        if (!cancelled) {
          setSnapshot(buildWorkspaceSettingsSnapshot(result));
          setFeedbackMessage("");
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setFeedbackMessage(getErrorMessage(
            error,
            t("settings.general.workspace_path_load_failed"),
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
      const result = await updateRuntimeSettingsApi({
        workspace_path: snapshot.draftPath.trim(),
      });
      setSnapshot(buildWorkspaceSettingsSnapshot(result));
      setFeedbackMessage(t("settings.general.workspace_path_saved"));
    } catch (error) {
      setFeedbackMessage(getErrorMessage(
        error,
        t("settings.general.workspace_path_save_failed"),
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
    save,
    saveDisabled: !canSaveWorkspaceSettings(snapshot, busy),
    saving,
    setDraftPath: (value: string) => {
      setSnapshot((current) => replaceWorkspaceDraft(current, value));
    },
  };
}
