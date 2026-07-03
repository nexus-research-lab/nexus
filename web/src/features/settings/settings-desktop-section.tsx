"use client";

import { useCallback, useEffect, useState } from "react";
import { Download, Loader2, MonitorCog } from "lucide-react";

import {
  exportDesktopLogs,
  getDesktopAppVersion,
  isDesktopBridgeAvailable,
  type DesktopAppVersion,
} from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";

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
} from "./settings-panel-ui";

export function SettingsDesktopSection() {
  const { t } = useI18n();
  const [desktopAvailable] = useState(() => isDesktopBridgeAvailable());
  const [desktopVersion, setDesktopVersion] =
    useState<DesktopAppVersion | null>(null);
  const [feedbackMessage, setFeedbackMessage] = useState<string | null>(
    null,
  );
  const [exportingLogs, setExportingLogs] = useState(false);

  useEffect(() => {
    if (!desktopAvailable) {
      return;
    }
    let cancelled = false;
    const loadVersion = async () => {
      try {
        const version = await getDesktopAppVersion();
        if (!cancelled) {
          setDesktopVersion(version);
        }
      } catch (error) {
        if (!cancelled) {
          setFeedbackMessage(
            error instanceof Error
              ? error.message
              : t("settings.desktop.version_failed"),
          );
        }
      }
    };
    void loadVersion();
    return () => {
      cancelled = true;
    };
  }, [desktopAvailable, t]);

  const handleExportLogs = useCallback(async () => {
    try {
      setExportingLogs(true);
      setFeedbackMessage(null);
      const result = await exportDesktopLogs();
      if (result.cancelled) {
        return;
      }
      setFeedbackMessage(
        result.path
          ? t("settings.desktop.export_logs_success_with_path").replace(
            "{path}",
            result.path,
          )
          : t("settings.desktop.export_logs_success"),
      );
    } catch (error) {
      setFeedbackMessage(
        error instanceof Error
          ? error.message
          : t("settings.desktop.export_logs_failed"),
      );
    } finally {
      setExportingLogs(false);
    }
  }, [t]);

  if (!desktopAvailable) {
    return null;
  }

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.desktop.section_title")}
        </h2>
        {feedbackMessage ? (
          <span className="min-w-0 truncate text-[11px] text-(--text-soft)">
            {feedbackMessage}
          </span>
        ) : null}
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
                {desktopVersion
                  ? t("settings.desktop.version_value")
                    .replace("{version}", desktopVersion.app_version)
                    .replace("{build}", desktopVersion.build_number)
                  : t("settings.desktop.version_loading")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
            <button
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:opacity-(--disabled-opacity)`}
              disabled={exportingLogs}
              onClick={handleExportLogs}
              type="button"
            >
              {exportingLogs ? <Loader2 className="h-3 w-3 animate-spin" /> : <Download className="h-3 w-3" />}
              {t("settings.desktop.export_logs")}
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
