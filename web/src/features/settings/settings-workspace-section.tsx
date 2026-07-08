"use client";

import { useCallback, useEffect, useState } from "react";
import { Folder, Loader2, Save } from "lucide-react";

import {
  getRuntimeSettingsApi,
  updateRuntimeSettingsApi,
} from "@/lib/api/settings-runtime-api";
import { UiInput } from "@/shared/ui/form-control";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { RuntimeSettings } from "@/types/settings/runtime";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_TEXT_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "./settings-panel-ui";

export function SettingsWorkspaceSection() {
  const { t } = useI18n();
  const [settings, setSettings] = useState<RuntimeSettings | null>(null);
  const [workspacePath, setWorkspacePath] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [feedbackMessage, setFeedbackMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const loadSettings = async () => {
      try {
        setLoading(true);
        const result = await getRuntimeSettingsApi();
        if (cancelled) {
          return;
        }
        setSettings(result);
        setWorkspacePath(result.workspace_path?.trim() ?? "");
      } catch (error) {
        if (!cancelled) {
          setFeedbackMessage(
            error instanceof Error
              ? error.message
              : t("settings.general.workspace_path_load_failed"),
          );
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };
    void loadSettings();
    return () => {
      cancelled = true;
    };
  }, [t]);

  const handleSave = useCallback(async () => {
    try {
      setSaving(true);
      setFeedbackMessage(null);
      const result = await updateRuntimeSettingsApi({
        workspace_path: workspacePath.trim(),
      });
      setSettings(result);
      setWorkspacePath(result.workspace_path?.trim() ?? "");
      setFeedbackMessage(t("settings.general.workspace_path_saved"));
    } catch (error) {
      setFeedbackMessage(
        error instanceof Error
          ? error.message
          : t("settings.general.workspace_path_save_failed"),
      );
    } finally {
      setSaving(false);
    }
  }, [t, workspacePath]);

  const savedWorkspacePath = settings?.workspace_path?.trim() ?? "";
  const currentWorkspacePath = settings?.current_workspace_path?.trim() ?? "";
  const saveDisabled =
    loading || saving || workspacePath.trim() === savedWorkspacePath;

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.general.section_workspace")}
        </h2>
        {feedbackMessage ? (
          <span className="min-w-0 truncate text-[11px] text-(--text-soft)">
            {feedbackMessage}
          </span>
        ) : null}
      </div>
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <div className="grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_minmax(260px,360px)] md:items-center">
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <Folder className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.workspace_path_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.workspace_path_description")}
              </p>
              {currentWorkspacePath ? (
                <p
                  className="mt-1 max-w-[520px] truncate font-mono text-[11px] text-(--text-muted)"
                  title={currentWorkspacePath}
                >
                  {t("settings.general.workspace_path_current", {
                    path: currentWorkspacePath,
                  })}
                </p>
              ) : null}
            </div>
          </div>
          <div className="flex min-w-0 items-center gap-2">
            <UiInput
              aria-label={t("settings.general.workspace_path_title")}
              className="font-mono"
              controlSize="sm"
              disabled={loading || saving}
              onChange={(event) => setWorkspacePath(event.target.value)}
              placeholder={t("settings.general.workspace_path_placeholder")}
              value={workspacePath}
              variant="surface"
            />
            <button
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex shrink-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:opacity-(--disabled-opacity)`}
              disabled={saveDisabled}
              onClick={handleSave}
              title={t("common.save")}
              type="button"
            >
              {saving ? <Loader2 className="h-3 w-3 animate-spin" /> : <Save className="h-3 w-3" />}
              {t("common.save")}
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
