/**
 * INPUT: 当前数据目录快照、用户草稿和迁移恢复反馈。
 * OUTPUT: 数据目录表单、迁移确认及结果未知时的对账入口。
 * POS: General 工作区分区视图；不推断迁移结果或重复发起迁移。
 */
"use client";

import { useState } from "react";
import { Folder, FolderOpen, Loader2, RefreshCw } from "lucide-react";

import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiInput } from "@/shared/ui/form/form-control";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_TEXT_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";
import { useWorkspaceSettings } from "../use-workspace-settings";

export function SettingsWorkspaceSection() {
  const { t } = useI18n();
  const controller = useWorkspaceSettings();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const titleKey = "settings.general.state_root_title";

  return (
    <>
      <section className="space-y-2.5">
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <div className="grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_minmax(260px,360px)] md:items-center">
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <Folder className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t(titleKey)}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.general.state_root_description")}
                </p>
                {controller.currentPath ? (
                  <p
                    className="mt-1 max-w-[520px] break-all font-mono text-xs leading-5 text-(--text-muted)"
                    title={controller.currentPath}
                  >
                    {t("settings.general.state_root_current", {
                      path: controller.currentPath,
                    })}
                  </p>
                ) : null}
              </div>
            </div>
            <div className="flex min-w-0 items-center gap-2">
              <div className="relative min-w-0 flex-1">
                <UiInput
                  aria-label={t(titleKey)}
                  className="pr-9 font-mono"
                  controlSize="sm"
                  disabled={controller.busy}
                  onChange={(event) => controller.setDraftPath(event.target.value)}
                  placeholder={controller.placeholder}
                  value={controller.draftPath}
                  variant="surface"
                />
                <button
                  aria-label={t("settings.general.state_root_select_action")}
                  className="absolute right-1 top-1/2 inline-flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-[7px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                  disabled={controller.busy}
                  onClick={() => void controller.selectDirectory()}
                  title={t("settings.general.state_root_select_action")}
                  type="button"
                >
                  {controller.selecting ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <FolderOpen className="h-3.5 w-3.5" />
                  )}
                </button>
              </div>
              <button
                className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex shrink-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:opacity-(--disabled-opacity)`}
                disabled={controller.saveDisabled}
                onClick={() => setConfirmOpen(true)}
                title={t("settings.general.state_root_action")}
                type="button"
              >
                {controller.saving ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <RefreshCw className="h-3 w-3" />
                )}
                {t("settings.general.state_root_action")}
              </button>
            </div>
          </div>
        </div>
      </section>
      <ConfirmDialog
        cancelText={t("common.cancel")}
        confirmText={t("settings.general.state_root_confirm_action")}
        isOpen={confirmOpen && !controller.saving}
        message={t("settings.general.state_root_confirm_message", {
          path: controller.draftPath.trim(),
        })}
        onCancel={() => setConfirmOpen(false)}
        onConfirm={() => {
          setConfirmOpen(false);
          void controller.save();
        }}
        title={t("settings.general.state_root_confirm_title")}
      />
      <FeedbackBannerViewport item={controller.feedback} />
    </>
  );
}
