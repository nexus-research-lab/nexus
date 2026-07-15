"use client";

import { Search, Terminal } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";

import { AGENT_RUNTIME_KIND_OPTIONS } from "./model/settings-runtime-options";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsSegmentedControl,
} from "../shared/settings-panel-ui";
import { useRuntimeSettingsController } from "./use-runtime-settings-controller";

export function SettingsRuntimeSection() {
  const { t } = useI18n();
  const settings = useRuntimeSettingsController();

  return (
    <div
      className={cn(
        "mx-auto flex w-full flex-col gap-5 px-1 py-3",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}
    >
      <section className="space-y-2.5">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.runtime.section_title")}
        </h2>
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <Terminal className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.runtime.kernel_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.runtime.kernel_description")}
                </p>
              </div>
            </div>
            <div className="flex min-w-0 flex-col gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.runtime.kernel_label")}
              </span>
              <SettingsSegmentedControl
                ariaLabel={t("settings.runtime.kernel_label")}
                disabled={
                  settings.loading ||
                  settings.preferencesBusy ||
                  settings.nxsRuntimeChecking
                }
                onChange={settings.onRuntimeKindChange}
                options={AGENT_RUNTIME_KIND_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.labelKey),
                }))}
                value={settings.runtimeKind}
              />
            </div>
          </div>

          {settings.runtimeKind === "nxs" ? (
            <>
              <div className="border-t border-(--divider-subtle-color)" />
              <ToolSearchRow
                checked={settings.toolSearchEnabled}
                disabled={settings.loading || settings.preferencesBusy}
                onChange={settings.onToolSearchChange}
              />
            </>
          ) : (
            <>
              <div className="border-t border-(--divider-subtle-color)" />
              <RuntimeWithoutSettings />
            </>
          )}
        </div>
        {settings.feedbackMessage ? (
          <p className="px-1 text-xs text-(--danger-text-color)">
            {settings.feedbackMessage}
          </p>
        ) : null}
      </section>
    </div>
  );
}

function ToolSearchRow({
  checked,
  disabled,
  onChange,
}: {
  checked: boolean;
  disabled: boolean;
  onChange: (checked: boolean) => void;
}) {
  const { t } = useI18n();
  return (
    <div className={SETTINGS_ROW_CLASS_NAME}>
      <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
        <div className={SETTINGS_ICON_CLASS_NAME}>
          <Search className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0">
          <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
            {t("settings.runtime.tool_search_title")}
          </h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {t("settings.runtime.tool_search_description")}
          </p>
        </div>
      </div>
      <div className="flex min-w-0 items-center justify-between gap-3 md:justify-end">
        <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
          {t("settings.runtime.tool_search_label")}
        </span>
        <GlassSwitch
          checked={checked}
          disabled={disabled}
          onChange={onChange}
          size="sm"
        />
      </div>
    </div>
  );
}

function RuntimeWithoutSettings() {
  const { t } = useI18n();
  return (
    <div className={SETTINGS_ROW_CLASS_NAME}>
      <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
        <div className={SETTINGS_ICON_CLASS_NAME}>
          <Terminal className="h-3.5 w-3.5" />
        </div>
        <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
          {t("settings.runtime.no_settings")}
        </p>
      </div>
    </div>
  );
}
