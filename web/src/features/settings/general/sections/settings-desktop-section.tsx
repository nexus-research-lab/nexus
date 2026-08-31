/**
 * INPUT: 桌面版本资源与日志导出反馈。
 * OUTPUT: 可重试版本状态、日志导出按钮及不裁切的全局反馈。
 * POS: General 桌面分区视图；不读取原生异常或改变 bridge 命令。
 */
"use client";

import { Download, Loader2, MonitorCog } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_TEXT_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";
import { useDesktopSettings } from "../use-desktop-settings";

export function SettingsDesktopSection() {
  const { t } = useI18n();
  const controller = useDesktopSettings();

  if (!controller.available) {
    return null;
  }

  return (
    <>
      <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.desktop.section_title")}
        </h2>
      </div>
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <MonitorCog className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.desktop.version_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {controller.versionDescription}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
            <button
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:opacity-(--disabled-opacity)`}
              disabled={controller.exportingLogs}
              onClick={() => void controller.exportLogs()}
              type="button"
            >
              {controller.exportingLogs ? <Loader2 className="h-3 w-3 animate-spin" /> : <Download className="h-3 w-3" />}
              {t("settings.desktop.export_logs")}
            </button>
          </div>
        </div>
        {controller.versionFailed ? (
          <>
            <div className="border-t border-(--divider-subtle-color)" />
            <UiResourceState
              className="border-0 bg-transparent"
              description={t("settings.desktop.version_failed")}
              impact={t("settings.desktop.version_failed_impact")}
              nextStep={t("settings.desktop.version_failed_next_step")}
              primaryAction={{
                busy: controller.versionLoading,
                busyLabel: t("settings.desktop.version_loading"),
                label: t("settings.desktop.version_retry"),
                onClick: controller.retryVersion,
              }}
              size="sm"
              state="error"
              title={t("settings.desktop.version_failed_title")}
              urgency="polite"
            />
          </>
        ) : null}
      </div>
      </section>
      <FeedbackBannerViewport item={controller.feedback} />
    </>
  );
}
