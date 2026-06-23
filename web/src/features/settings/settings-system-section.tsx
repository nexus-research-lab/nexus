"use client";

import { useEffect, useState } from "react";
import { ExternalLink, PackageOpen } from "lucide-react";

import {
  get_system_version_api,
  type SystemVersionInfo,
} from "@/lib/api/system-api";
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

const DEFAULT_RELEASE_PAGE_URL =
  "https://github.com/nexus-research-lab/nexus/releases/latest";

export function SettingsSystemSection() {
  const { t } = useI18n();
  const [system_version, set_system_version] =
    useState<SystemVersionInfo | null>(null);
  const [loading, set_loading] = useState(true);
  const [feedback_message, set_feedback_message] = useState<string | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;
    const load_system_version = async () => {
      try {
        set_loading(true);
        const result = await get_system_version_api();
        if (cancelled) {
          return;
        }
        set_system_version(result);
        set_feedback_message(null);
      } catch (error) {
        if (!cancelled) {
          set_feedback_message(
            error instanceof Error
              ? error.message
              : t("settings.system.version_failed"),
          );
        }
      } finally {
        if (!cancelled) {
          set_loading(false);
        }
      }
    };
    void load_system_version();
    return () => {
      cancelled = true;
    };
  }, [t]);

  const release_page_url =
    system_version?.release_url || DEFAULT_RELEASE_PAGE_URL;
  const version_description = system_version
    ? t("settings.system.version_value")
      .replace("{version}", system_version.version)
      .replace("{target}", system_version.target)
    : loading
      ? t("settings.system.version_loading")
      : t("settings.system.version_unavailable");

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.system.section_title")}
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
              <PackageOpen className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.system.version_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {version_description}
              </p>
            </div>
          </div>
          <a
            className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)`}
            href={release_page_url}
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
