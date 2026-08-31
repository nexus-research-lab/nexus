// INPUT: 当前 Session、本机目录选择器及目录设置读写 API。
// OUTPUT: 保留最后列表的目录控制器，以及按 mutation effect 投影的恢复锁。
// POS: Composer 本机目录设置边界；保存未知只允许重新读取，不重复写入。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import {
  getSessionLocalDirectoriesApi,
  updateSessionLocalDirectoriesApi,
} from "@/lib/api/conversation/session-api";
import {
  chooseDesktopDirectory,
  isDesktopBridgeAvailable,
} from "@/lib/desktop-bridge/desktop-bridge";
import {
  getResourceFailure,
  projectMutationFailure,
} from "@/lib/error-message";
import { subscribeSessionRuntimeSettingsUpdated } from "@/lib/conversation/session-runtime-settings-events";
import { useI18n } from "@/shared/i18n/i18n-context";

export function useComposerLocalDirectories(sessionKey?: string) {
  const { t } = useI18n();
  const normalizedSessionKey = sessionKey?.trim() ?? "";
  const nativePickerAvailable = isDesktopRuntime()
    && isDesktopBridgeAvailable();
  const available = Boolean(normalizedSessionKey) && nativePickerAvailable;
  const activeSessionKeyRef = useRef(normalizedSessionKey);
  activeSessionKeyRef.current = normalizedSessionKey;
  const [directories, setDirectories] = useState<string[]>([]);
  const [failure, setFailure] = useState<ComposerLocalDirectoryFailure | null>(
    null,
  );
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revision, setRevision] = useState(0);

  useEffect(() => setSaving(false), [normalizedSessionKey]);

  useEffect(() => {
    if (!available) {
      setDirectories([]);
      setFailure(null);
      setLoading(false);
      return undefined;
    }
    let active = true;
    setLoading(true);
    setFailure(null);
    void getSessionLocalDirectoriesApi(normalizedSessionKey)
      .then((result) => {
        if (active) setDirectories(result.directories ?? []);
      })
      .catch((requestError: unknown) => {
        if (active) {
          const resourceFailure = getResourceFailure(
            requestError,
            t("composer.local_directories_load_failed"),
          );
          setFailure({
            blocksMutation: false,
            impact: t("composer.local_directories_load_failed_impact"),
            message: resourceFailure.message,
            nextStep: t("composer.local_directories_load_failed_next_step"),
          });
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [available, normalizedSessionKey, revision, t]);

  useEffect(() => subscribeSessionRuntimeSettingsUpdated((updatedKey) => {
    if (updatedKey === activeSessionKeyRef.current) {
      setRevision((current) => current + 1);
    }
  }), []);

  const save = useCallback(async (nextDirectories: string[]) => {
    if (!available || saving || failure?.blocksMutation) return;
    const targetSessionKey = normalizedSessionKey;
    setSaving(true);
    setFailure(null);
    try {
      const result = await updateSessionLocalDirectoriesApi(
        targetSessionKey,
        nextDirectories,
      );
      if (activeSessionKeyRef.current === targetSessionKey) {
        setDirectories(result.directories ?? []);
      }
    } catch (requestError) {
      if (activeSessionKeyRef.current === targetSessionKey) {
        const mutationFailure = projectMutationFailure(
          requestError,
          t("composer.local_directories_save_failed"),
        );
        const notApplied = mutationFailure.effect === "not_applied";
        setFailure({
          blocksMutation: !notApplied,
          impact: t(notApplied
            ? "composer.local_directories_save_not_applied_impact"
            : "composer.local_directories_save_unknown_impact"),
          message: mutationFailure.message,
          nextStep: t(notApplied
            ? "composer.local_directories_save_not_applied_next_step"
            : "composer.local_directories_save_unknown_next_step"),
        });
      }
    } finally {
      setSaving(false);
    }
  }, [available, failure?.blocksMutation, normalizedSessionKey, saving, t]);

  const chooseDirectory = useCallback(async () => {
    if (!available || saving) return;
    let selectedPath = "";
    try {
      const result = await chooseDesktopDirectory(
        directories.at(-1) ?? "",
        t("composer.local_directory_picker_title"),
        t("composer.local_directory_picker_prompt"),
      );
      selectedPath = result.cancelled ? "" : result.path?.trim() ?? "";
    } catch (pickerError) {
      setFailure({
        blocksMutation: false,
        impact: t("composer.local_directory_picker_failed_impact"),
        message: getResourceFailure(
          pickerError,
          t("composer.local_directory_picker_failed"),
        ).message,
        nextStep: t("composer.local_directory_picker_failed_next_step"),
      });
      return;
    }
    if (!selectedPath || directories.includes(selectedPath)) return;
    await save([...directories, selectedPath]);
  }, [available, directories, save, saving, t]);

  return {
    available,
    chooseDirectory,
    directories,
    failure,
    loading,
    removeDirectory: (directory: string) => save(
      directories.filter((candidate) => candidate !== directory),
    ),
    reload: () => setRevision((current) => current + 1),
    saving,
};
}

export type ComposerLocalDirectoriesController = ReturnType<
  typeof useComposerLocalDirectories
>;

export interface ComposerLocalDirectoryFailure {
  blocksMutation: boolean;
  impact: string;
  message: string;
  nextStep: string;
}
