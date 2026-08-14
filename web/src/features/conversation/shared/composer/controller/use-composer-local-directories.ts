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
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revision, setRevision] = useState(0);

  useEffect(() => setSaving(false), [normalizedSessionKey]);

  useEffect(() => {
    if (!available) {
      setDirectories([]);
      setError(null);
      setLoading(false);
      return undefined;
    }
    let active = true;
    setLoading(true);
    setError(null);
    void getSessionLocalDirectoriesApi(normalizedSessionKey)
      .then((result) => {
        if (active) setDirectories(result.directories ?? []);
      })
      .catch((requestError: unknown) => {
        if (active) {
          setError(resolveErrorMessage(
            requestError,
            t("composer.local_directories_load_failed"),
          ));
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
    if (!available || saving) return;
    const targetSessionKey = normalizedSessionKey;
    setSaving(true);
    setError(null);
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
        setError(resolveErrorMessage(
          requestError,
          t("composer.local_directories_save_failed"),
        ));
      }
    } finally {
      setSaving(false);
    }
  }, [available, normalizedSessionKey, saving, t]);

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
      setError(resolveErrorMessage(
        pickerError,
        t("composer.local_directories_save_failed"),
      ));
      return;
    }
    if (!selectedPath || directories.includes(selectedPath)) return;
    await save([...directories, selectedPath]);
  }, [available, directories, save, saving, t]);

  return {
    available,
    chooseDirectory,
    directories,
    error,
    loading,
    removeDirectory: (directory: string) => save(
      directories.filter((candidate) => candidate !== directory),
    ),
    saving,
  };
}

export type ComposerLocalDirectoriesController = ReturnType<
  typeof useComposerLocalDirectories
>;

function resolveErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim()
    ? error.message
    : fallback;
}
