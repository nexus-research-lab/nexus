"use client";

import { ExternalLink, PackageOpen } from "lucide-react";

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
} from "../../shared/settings-panel-ui";
import { useSystemSettings } from "../use-system-settings";

export function SettingsSystemSection() {
  const { t } = useI18n();
  const controller = useSystemSettings();

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.system.section_title")}
        </h2>
        {controller.feedbackMessage ? (
          <span className="min-w-0 truncate text-[11px] text-(--text-soft)">
            {controller.feedbackMessage}
          </span>
        ) : null}
      </div>
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <PackageOpen className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.system.version_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {controller.versionDescription}
              </p>
            </div>
          </div>
          <a
            className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)`}
            href={controller.releasePageUrl}
            rel="noreferrer"
            target="_blank"
          >
            <ExternalLink className="h-3 w-3" />
            {t("settings.system.download_release")}
          </a>
        </div>
      </div>
    </section>
  );
}
