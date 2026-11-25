// INPUT: 宿主平台、组件状态、运行时报告的权限及设置操作。
// OUTPUT: 电脑操控开关、对齐在开关正文下的系统权限摘要与 macOS 原生授权向导入口。
// POS: 设置域的电脑操控入口，Windows 在此查看状态。
"use client";

import type { ReactNode } from "react";
import { AlertTriangle, Check, CircleDashed, Minus, MousePointer2, RefreshCw, X } from "lucide-react";

import { getDesktopPlatform } from "@/config/desktop-runtime";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { getUiTypographyClassName, type UiTypographyTone } from "@/shared/ui/typography/typography-styles";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTENT_BODY_CLASS_NAME,
  SettingsToggleRow,
} from "../shared/settings-panel-ui";
import { PERMISSION_ORDER, useComputerUseSettingsController } from "./use-computer-use-settings-controller";

type ComputerUseSettings = ReturnType<typeof useComputerUseSettingsController>;

interface PermissionStateView {
  icon: ReactNode;
  labelKey: TranslationKey;
  // 已开启只用图标表达，文字留给读屏；需要处理的状态才显示文字。
  quiet: boolean;
  tone: UiTypographyTone;
}

const ICON_CLASS_NAME = "h-3.5 w-3.5 shrink-0";

const PERMISSION_STATES: Record<string, PermissionStateView> = {
  granted: { icon: <Check className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_granted", quiet: true, tone: "success" },
  not_applicable: { icon: <Minus className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_not_applicable", quiet: false, tone: "soft" },
  denied: { icon: <X className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_denied", quiet: false, tone: "danger" },
  not_determined: { icon: <CircleDashed className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_not_determined", quiet: false, tone: "warning" },
};
const UNKNOWN_PERMISSION_STATE: PermissionStateView = {
  icon: <CircleDashed className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_unknown", quiet: false, tone: "warning",
};

const SATISFIED_PERMISSIONS = new Set(["granted", "not_applicable"]);

const ERROR_KEYS = {
  component: "settings.computer_use.failed",
  check: "settings.computer_use.check_failed",
  setup: "settings.computer_use.setup_failed",
} as const;

const META_CLASS_NAME = getUiTypographyClassName({ role: "metadata", tone: "muted" });

function PermissionItem({ name, value }: { name: TranslationKey; value?: string }) {
  const { t } = useI18n();
  const state = PERMISSION_STATES[value ?? ""] ?? UNKNOWN_PERMISSION_STATE;
  return (
    <li className="flex items-center gap-1.5">
      <span aria-hidden="true" className={getUiTypographyClassName({ role: "metadata", tone: state.tone })}>
        {state.icon}
      </span>
      <span className={getUiTypographyClassName({ role: "metadata", tone: "default" })}>{t(name)}</span>
      <span className={cn(
        state.quiet && "sr-only",
        getUiTypographyClassName({ role: "metadata", tone: state.tone }),
      )}>
        {t(state.labelKey)}
      </span>
    </li>
  );
}

function PermissionSummary({ settings }: { settings: ComputerUseSettings }) {
  const { t } = useI18n();
  const permissions = settings.status?.permissions;
  const missing = PERMISSION_ORDER.filter((key) => !SATISFIED_PERMISSIONS.has(permissions?.[key] ?? ""));
  // Windows 按应用授权，不存在全局开关，不再罗列三个“不适用”。
  if (PERMISSION_ORDER.every((key) => permissions?.[key] === "not_applicable")) {
    return <p className={META_CLASS_NAME} role="status">{t("settings.computer_use.permissions_not_required")}</p>;
  }
  const canSetup = missing.length > 0 && getDesktopPlatform() === "macos";
  return (
    <div className="space-y-1.5">
      <div className="flex min-h-7 flex-wrap items-center gap-x-2 gap-y-1">
        <h4 className={getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "medium" })}>
          {t("settings.computer_use.permissions_title")}
        </h4>
        <span
          className={getUiTypographyClassName({ role: "metadata", tone: missing.length > 0 ? "warning" : "success" })}
          role="status"
        >
          {missing.length > 0
            ? t("settings.computer_use.permissions_missing", { count: missing.length })
            : t("settings.computer_use.permissions_ready")}
        </span>
        <div className="ml-auto flex items-center gap-1.5">
          <UiButton disabled={settings.busy} onClick={() => void settings.onCheckPermissions()} size="xs" variant="text">
            <RefreshCw aria-hidden="true" className="h-3 w-3" />
            {t("settings.computer_use.check_permissions")}
          </UiButton>
          {canSetup && (
            <UiButton disabled={settings.busy} onClick={() => void settings.onOpenPermissions()} size="xs" tone="primary" variant="solid">
              {t("settings.computer_use.open_permissions")}
            </UiButton>
          )}
        </div>
      </div>
      <ul aria-label={t("settings.computer_use.permissions_title")} className="flex flex-wrap gap-x-5 gap-y-1">
        {PERMISSION_ORDER.map((key) => (
          <PermissionItem key={key} name={`settings.computer_use.permission_${key}`} value={permissions?.[key]} />
        ))}
      </ul>
    </div>
  );
}

function ComputerUseError({ settings }: { settings: ComputerUseSettings }) {
  const { t } = useI18n();
  if (!settings.error) return null;
  const retry = settings.error === "check"
    ? { label: t("settings.computer_use.check_permissions"), onClick: () => void settings.onCheckPermissions() }
    : settings.error === "setup"
      ? { label: t("settings.computer_use.open_permissions"), onClick: () => void settings.onOpenPermissions() }
      : undefined;
  return (
    <UiInlineNotice
      action={retry && { ...retry, disabled: settings.busy }}
      icon={<AlertTriangle />}
      message={t(ERROR_KEYS[settings.error])}
      role="alert"
      tone="danger"
    />
  );
}

export function ComputerUseSettingsSection() {
  const { t, locale } = useI18n();
  const settings = useComputerUseSettingsController(locale);
  const status = settings.status;
  const enabled = status?.enabled === true;
  // 版本只作为开关说明的尾注；进行中的安装或检查直接替换说明，避免在卡片里插入会跳动的状态行。
  const description = settings.busy
    ? t("settings.computer_use.working")
    : !status?.installed
      ? t("settings.computer_use.install_description")
      : status.version
        ? t("settings.computer_use.description_version", { version: status.version })
        : t("settings.computer_use.description");
  const detail = status?.update_required ? (
    <UiInlineNotice
      action={{ disabled: settings.busy, label: t("settings.computer_use.update"), onClick: () => void settings.onUpdate() }}
      icon={<AlertTriangle />}
      message={t("settings.computer_use.update_required", { version: status.required_version ?? "" })}
      tone="warning"
    />
  ) : enabled ? <PermissionSummary settings={settings} /> : null;

  return (
    <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
      <WorkspaceContentHeader className="max-sm:hidden" title={t("settings.computer_use.section_title")} />
      <section className={SETTINGS_CONTENT_BODY_CLASS_NAME}>
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <SettingsToggleRow
            checked={enabled}
            description={description}
            disabled={settings.busy || !status}
            icon={<MousePointer2 className="h-3.5 w-3.5" />}
            onChange={settings.onChange}
            title={t("settings.computer_use.title")}
          />
          {(detail || settings.error) && (
            // 与开关标题左缘对齐（px-4 + 28px 图标 + 12px 间距），读作开关的下属状态。
            <div className="space-y-2 pb-3 pl-14 pr-4">
              {detail}
              <ComputerUseError settings={settings} />
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
