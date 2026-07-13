"use client";

import { Folder, Loader2, Save } from "lucide-react";

import { UiInput } from "@/shared/ui/form/form-control";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_TEXT_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";
import { useWorkspaceSettings } from "../use-workspace-settings";

export function SettingsWorkspaceSection() {
  const { t } = useI18n();
  const controller = useWorkspaceSettings();

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.general.section_workspace")}
        </h2>
        {controller.feedbackMessage ? (
          <span className="min-w-0 truncate text-[11px] text-(--text-soft)">
            {controller.feedbackMessage}
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
              {controller.currentPath ? (
                <p
                  className="mt-1 max-w-[520px] truncate font-mono text-[11px] text-(--text-muted)"
                  title={controller.currentPath}
                >
                  {t("settings.general.workspace_path_current", {
                    path: controller.currentPath,
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
              disabled={controller.busy}
              onChange={(event) => controller.setDraftPath(event.target.value)}
              placeholder={t("settings.general.workspace_path_placeholder")}
              value={controller.draftPath}
              variant="surface"
            />
            <button
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex shrink-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:opacity-(--disabled-opacity)`}
              disabled={controller.saveDisabled}
              onClick={() => void controller.save()}
              title={t("common.save")}
              type="button"
            >
              {controller.saving ? <Loader2 className="h-3 w-3 animate-spin" /> : <Save className="h-3 w-3" />}
              {t("common.save")}
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
