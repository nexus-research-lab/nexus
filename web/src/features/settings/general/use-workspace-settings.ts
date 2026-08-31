/**
 * INPUT: 桌面桥接返回的当前状态根、目录选择和整体迁移结果。
 * OUTPUT: 保留当前快照/草稿，并把读取、选择、迁移未知结果投影为可恢复反馈。
 * POS: 数据目录设置控制器；不改变迁移命令和路径身份，未知结果只做只读对账。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import { getDesktopRuntimeConfig } from "@/config/desktop-runtime/runtime-config";
import {
  chooseDesktopStateRoot,
  getDesktopStateRoot,
  relocateDesktopStateRoot,
} from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";

import {
  EMPTY_WORKSPACE_SETTINGS_SNAPSHOT,
  canSaveWorkspaceSettings,
  getStateRootPlaceholderKey,
  reconcileStateRootSettingsSnapshot,
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
  const [selecting, setSelecting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackBannerProps | null>(null);
  const [migrationUnconfirmed, setMigrationUnconfirmed] = useState(false);
  const selectingRef = useRef(false);
  const savingRef = useRef(false);
  const loadRequestRef = useRef(0);
  const loadingRef = useRef(false);
  const loadCurrentStateRef = useRef<() => void>(() => {});
  const migrationUnconfirmedRef = useRef(false);
  const pendingMigrationTargetRef = useRef<string | null>(null);

  const loadCurrentState = useCallback(() => {
    if (loadingRef.current) {
      return;
    }
    loadingRef.current = true;
    const requestId = loadRequestRef.current + 1;
    loadRequestRef.current = requestId;
    setLoading(true);
    void getDesktopStateRoot()
      .then((result) => {
        if (loadRequestRef.current !== requestId) {
          return;
        }
        const wasReconciling = pendingMigrationTargetRef.current !== null;
        setSnapshot((current) => reconcileStateRootSettingsSnapshot(
          current,
          result,
        ));
        pendingMigrationTargetRef.current = null;
        migrationUnconfirmedRef.current = false;
        setMigrationUnconfirmed(false);
        if (result.migration_error) {
          setFeedback({
            impact: t("settings.general.state_root_previous_failed_impact"),
            message: t("settings.general.state_root_previous_failed_message"),
            nextStep: t("settings.general.state_root_previous_failed_next_step"),
            title: t("settings.general.state_root_previous_failed_title"),
            tone: "error",
          });
          return;
        }
        setFeedback(wasReconciling
          ? {
              impact: t("settings.general.state_root_reconciled_impact"),
              message: t("settings.general.state_root_reconciled_message"),
              nextStep: t("settings.general.state_root_reconciled_next_step"),
              onDismiss: () => setFeedback(null),
              title: t("settings.general.state_root_reconciled_title"),
              tone: "success",
            }
          : null);
      })
      .catch(() => {
        if (loadRequestRef.current === requestId) {
          setFeedback({
            action: {
              label: t("settings.general.state_root_check_action"),
              onClick: () => loadCurrentStateRef.current(),
            },
            impact: migrationUnconfirmedRef.current
              ? t("settings.general.state_root_unknown_impact")
              : t("settings.general.state_root_load_failed_impact"),
            message: t("settings.general.state_root_load_failed"),
            nextStep: migrationUnconfirmedRef.current
              ? t("settings.general.state_root_unknown_next_step")
              : t("settings.general.state_root_load_failed_next_step"),
            title: migrationUnconfirmedRef.current
              ? t("settings.general.state_root_unknown_title")
              : t("settings.general.state_root_load_failed_title"),
            tone: migrationUnconfirmedRef.current ? "warning" : "error",
          });
        }
      })
      .finally(() => {
        if (loadRequestRef.current === requestId) {
          loadingRef.current = false;
          setLoading(false);
        }
      });
  }, [t]);
  loadCurrentStateRef.current = loadCurrentState;

  useEffect(() => {
    loadCurrentState();
    return () => {
      loadRequestRef.current += 1;
      loadingRef.current = false;
    };
  }, [loadCurrentState]);

  const selectDirectory = useCallback(async () => {
    if (selectingRef.current) {
      return;
    }
    selectingRef.current = true;
    setSelecting(true);
    setFeedback(null);
    try {
      const result = await chooseDesktopStateRoot(
        snapshot.draftPath.trim(),
        t("settings.general.state_root_select_title"),
        t("settings.general.state_root_select_action"),
      );
      const selectedPath = result.path?.trim() ?? "";
      if (!result.cancelled && selectedPath) {
        setSnapshot((current) => replaceWorkspaceDraft(current, selectedPath));
      }
    } catch {
      setFeedback({
        impact: t("settings.general.state_root_select_failed_impact"),
        message: t("settings.general.state_root_select_failed"),
        nextStep: t("settings.general.state_root_select_failed_next_step"),
        title: t("settings.general.state_root_select_failed_title"),
        tone: "error",
      });
    } finally {
      selectingRef.current = false;
      setSelecting(false);
    }
  }, [snapshot.draftPath, t]);

  const save = useCallback(async () => {
    if (savingRef.current) {
      return;
    }
    savingRef.current = true;
    setSaving(true);
    setFeedback(null);
    const targetPath = snapshot.draftPath.trim();
    try {
      await relocateDesktopStateRoot(targetPath);
      setFeedback({
        message: t("settings.general.state_root_restarting"),
        title: t("settings.general.state_root_restarting_title"),
        tone: "success",
      });
    } catch {
      pendingMigrationTargetRef.current = targetPath;
      migrationUnconfirmedRef.current = true;
      setMigrationUnconfirmed(true);
      setFeedback({
        action: {
          label: t("settings.general.state_root_check_action"),
          onClick: () => loadCurrentStateRef.current(),
        },
        impact: t("settings.general.state_root_unknown_impact"),
        message: t("settings.general.state_root_save_failed"),
        nextStep: t("settings.general.state_root_unknown_next_step"),
        title: t("settings.general.state_root_unknown_title"),
        tone: "warning",
      });
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }, [snapshot.draftPath, t]);

  const busy = loading || selecting || saving;
  return {
    busy,
    currentPath: snapshot.currentPath,
    draftPath: snapshot.draftPath,
    feedback,
    placeholder,
    save,
    saveDisabled: !canSaveWorkspaceSettings(
      snapshot,
      busy || migrationUnconfirmed,
    ),
    selectDirectory,
    selecting,
    saving,
    setDraftPath: (value: string) => {
      setSnapshot((current) => replaceWorkspaceDraft(current, value));
    },
  };
}
