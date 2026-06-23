"use client";

import { useCallback, useEffect, useState } from "react";
import { Download, Loader2, MonitorCog } from "lucide-react";

import {
  export_desktop_logs,
  get_desktop_app_version,
  is_desktop_bridge_available,
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
  const [desktop_available] = useState(() => is_desktop_bridge_available());
  const [desktop_version, set_desktop_version] =
    useState<DesktopAppVersion | null>(null);
  const [feedback_message, set_feedback_message] = useState<string | null>(
    null,
  );
  const [exporting_logs, set_exporting_logs] = useState(false);

  useEffect(() => {
    if (!desktop_available) {
      return;
    }
    let cancelled = false;
    const load_version = async () => {
      try {
        const version = await get_desktop_app_version();
        if (!cancelled) {
          set_desktop_version(version);
        }
      } catch (error) {
        if (!cancelled) {
          set_feedback_message(
            error instanceof Error
              ? error.message
              : t("settings.desktop.version_failed"),
          );
        }
      }
    };
    void load_version();
    return () => {
      cancelled = true;
    };
  }, [desktop_available, t]);

  const handle_export_logs = useCallback(async () => {
    try {
      set_exporting_logs(true);
      set_feedback_message(null);
      const result = await export_desktop_logs();
      if (result.cancelled) {
        return;
      }
      set_feedback_message(
        result.path
          ? t("settings.desktop.export_logs_success_with_path").replace(
            "{path}",
            result.path,
          )
          : t("settings.desktop.export_logs_success"),
      );
    } catch (error) {
      set_feedback_message(
        error instanceof Error
          ? error.message
          : t("settings.desktop.export_logs_failed"),
      );
    } finally {
      set_exporting_logs(false);
    }
  }, [t]);

  if (!desktop_available) {
    return null;
  }

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.desktop.section_title")}
        </h2>
        {feedback_message ? (
          <span className="min-w-0 truncate text-[11px] text-(--text-soft)">
            {feedback_message}
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
                {desktop_version
                  ? t("settings.desktop.version_value")
                    .replace("{version}", desktop_version.app_version)
                    .replace("{build}", desktop_version.build_number)
                  : t("settings.desktop.version_loading")}
              </p>
            </div>
          </div>
          <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
            <button
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:opacity-(--disabled-opacity)`}
              disabled={exporting_logs}
              onClick={handle_export_logs}
              type="button"
            >
              {exporting_logs ? <Loader2 className="h-3 w-3 animate-spin" /> : <Download className="h-3 w-3" />}
              {t("settings.desktop.export_logs")}
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
