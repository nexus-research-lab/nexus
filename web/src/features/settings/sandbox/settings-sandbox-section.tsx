/**
 * INPUT: 沙箱设置控制器提供的桌面沙箱偏好与诊断状态。
 * OUTPUT: 桌面沙箱开关与显式“检查”诊断；两行控件统一靠右、垂直居中，结果按语义 tone 展示。
 * POS: 设置目录的沙箱分区，仅桌面端可见；不直接调用 API。
 */
"use client";

import { Box, Loader2, ScanSearch, ShieldCheck } from "lucide-react";
import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { getUiTypographyClassName, type UiTypographyTone } from "@/shared/ui/typography/typography-styles";
import type { NXSSandboxDiagnosticState } from "@/types/settings/preferences";

import { PreferencesReliabilityNotice } from "../general/components/preferences-reliability-notice";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTENT_BODY_CLASS_NAME,
  SETTINGS_DIVIDER_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsToggleRow,
} from "../shared/settings-panel-ui";
import { useSandboxSettingsController } from "./use-sandbox-settings-controller";

// 诊断结果的语义色：只有依赖齐全是成功，未知保持中性，其余提示需要处理。
const SANDBOX_STATE_TONES = {
  dependencies_available: "success",
  missing_dependencies: "warning",
  unknown: "muted",
  unsupported: "warning",
} as const satisfies Record<NXSSandboxDiagnosticState, UiTypographyTone>;

function resultClassName(tone: UiTypographyTone) {
  return cn("mt-1", getUiTypographyClassName({ role: "metadata", tone }));
}

export function SettingsSandboxSection() {
  const { t } = useI18n();
  const settings = useSandboxSettingsController();

  return (
    <div className={cn(WORKSPACE_CONTENT_PAGE_CLASS_NAME, "flex flex-col")}>
      <WorkspaceContentHeader
        className="max-sm:hidden"
        title={t("settings.sandbox.section_title")}
      />
      <section className={cn(SETTINGS_CONTENT_BODY_CLASS_NAME, "space-y-2.5")}>
        <PreferencesReliabilityNotice
          feedback={settings.preferencesFeedback}
          recovery={settings.preferencesRecovery}
        />
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <SettingsToggleRow
            checked={settings.desktopSandboxEnabled}
            description={t("settings.general.desktop_sandbox_description")}
            disabled={settings.loading || settings.preferencesBusy}
            icon={<Box className="h-3.5 w-3.5" />}
            onChange={settings.onDesktopSandboxEnabledChange}
            title={t("settings.general.desktop_sandbox_title")}
          />
          <div className={SETTINGS_DIVIDER_CLASS_NAME} />
          <SandboxActionRow
            actions={(
              <UiButton
                aria-busy={settings.sandboxChecking}
                disabled={settings.sandboxChecking}
                onClick={() => void settings.onCheckSandbox()}
                size="xs"
                variant="surface"
              >
                {settings.sandboxChecking
                  ? <Loader2 className={getUiSpinnerClassName({ size: "xs" })} />
                  : <ScanSearch className="h-3 w-3" />}
                {t(settings.sandboxChecking ? "settings.runtime.sandbox_checking" : "settings.runtime.sandbox_check_action")}
              </UiButton>
            )}
            description={t("settings.runtime.sandbox_check_description")}
            icon={<ShieldCheck className="h-3.5 w-3.5" />}
            title={t("settings.runtime.sandbox_check")}
          >
            {settings.sandboxState ? (
              <p role="status" className={resultClassName(SANDBOX_STATE_TONES[settings.sandboxState])}>
                {settings.sandboxMessage || t(`settings.runtime.sandbox_${settings.sandboxState}`)}
              </p>
            ) : null}
          </SandboxActionRow>
        </div>
      </section>
    </div>
  );
}

// 与开关行同一几何：文字区占满剩余宽度，动作贴右并与首行标题区垂直居中。
function SandboxActionRow({
  actions,
  children,
  description,
  icon,
  title,
}: {
  actions: ReactNode;
  children?: ReactNode;
  description: string;
  icon: ReactNode;
  title: string;
}) {
  return (
    <div className="flex min-w-0 items-center gap-3 px-4 py-3">
      <div className={cn(SETTINGS_TEXT_ROW_CLASS_NAME, "flex-1")}>
        <div aria-hidden="true" className={SETTINGS_ICON_CLASS_NAME}>{icon}</div>
        <div className="min-w-0 break-words">
          <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>{title}</h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>{description}</p>
          {children}
        </div>
      </div>
      <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">{actions}</div>
    </div>
  );
}
