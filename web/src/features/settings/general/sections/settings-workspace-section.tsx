/**
 * INPUT: 当前数据目录快照、用户草稿和迁移恢复反馈。
 * OUTPUT: 数据目录表单、迁移确认及结果未知时的对账入口。
 * POS: General 工作区分区视图；不推断迁移结果或重复发起迁移。
 */
"use client";

import { useState } from "react";
import {
  Folder,
  FolderOpen,
  Loader2,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";

import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiInput } from "@/shared/ui/form/form-control";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import {
  SETTINGS_CARD_CLASS_NAME,
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
                {controller.showAdministratorNotice ? (
                  <p
                    className={cn(
                      "mt-1 flex max-w-[520px] items-start gap-1.5",
                      getUiTypographyClassName({ role: "caption", tone: "warning" }),
                    )}
                    role="alert"
                  >
                    <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    <span>
                      {t("settings.general.state_root_windows_admin_notice")}
                    </span>
                  </p>
                ) : null}
                {controller.currentPath ? (
                  <p
                    className={cn(
                      "mt-1 max-w-[520px] break-all",
                      getUiTypographyClassName({ role: "code", tone: "muted" }),
                    )}
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
                  className={cn(
                    "pr-9",
                    getUiTypographyClassName({ role: "code" }),
                  )}
                  controlSize="sm"
                  disabled={controller.busy}
                  onChange={(event) => controller.setDraftPath(event.target.value)}
                  placeholder={controller.placeholder}
                  value={controller.draftPath}
                  variant="surface"
                />
                <UiIconButton
                  aria-label={t("settings.general.state_root_select_action")}
                  className="absolute right-1 top-1/2 -translate-y-1/2 disabled:pointer-events-none"
                  disabled={controller.busy}
                  onClick={() => void controller.selectDirectory()}
                  size="xs"
                  tooltip={t("settings.general.state_root_select_action")}
                  variant="ghost"
                >
                  {controller.selecting ? (
                    <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
                  ) : (
                    <FolderOpen className="h-3.5 w-3.5" />
                  )}
                </UiIconButton>
              </div>
              <UiButton
                className="shrink-0"
                disabled={controller.saveDisabled}
                onClick={() => setConfirmOpen(true)}
                size="xs"
                title={t("settings.general.state_root_action")}
                variant="surface"
              >
                {controller.saving ? (
                  <Loader2 className={getUiSpinnerClassName({ size: "xs" })} />
                ) : (
                  <RefreshCw className="h-3 w-3" />
                )}
                {t("settings.general.state_root_action")}
              </UiButton>
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
