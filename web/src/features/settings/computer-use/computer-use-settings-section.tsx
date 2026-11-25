// INPUT: 宿主平台、组件状态、运行时报告的权限及设置操作。
// OUTPUT: 复用设置详情 Pattern 的电脑操控页：标题与开关、示例横幅、能力说明、系统权限和组件信息。
// POS: 设置域的电脑操控入口，Windows 在此查看状态。
"use client";

import { useId, type ReactNode } from "react";
import { AlertTriangle, Check, CircleDashed, Minus, MousePointer2, RefreshCw, X } from "lucide-react";

import { getDesktopPlatform } from "@/config/desktop-runtime";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName, type UiTypographyTone } from "@/shared/ui/typography/typography-styles";

import {
  SETTINGS_DETAIL_BODY_CLASS_NAME,
  SettingsDetailExamples,
  SettingsDetailHeader,
  SettingsDetailInfo,
  SettingsDetailOverview,
  SettingsDetailSection,
} from "../shared/settings-detail-page";
import { SETTINGS_CARD_CLASS_NAME, SETTINGS_CONTENT_BODY_CLASS_NAME } from "../shared/settings-panel-ui";
import { PERMISSION_ORDER, useComputerUseSettingsController } from "./use-computer-use-settings-controller";

type ComputerUseSettings = ReturnType<typeof useComputerUseSettingsController>;

interface PermissionStateView {
  icon: ReactNode;
  labelKey: TranslationKey;
  tone: UiTypographyTone;
}

const ICON_CLASS_NAME = "h-3.5 w-3.5 shrink-0";

const PERMISSION_STATES: Record<string, PermissionStateView> = {
  granted: { icon: <Check className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_granted", tone: "success" },
  not_applicable: { icon: <Minus className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_not_applicable", tone: "soft" },
  denied: { icon: <X className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_denied", tone: "danger" },
  not_determined: { icon: <CircleDashed className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_not_determined", tone: "warning" },
};
const UNKNOWN_PERMISSION_STATE: PermissionStateView = {
  icon: <CircleDashed className={ICON_CLASS_NAME} />, labelKey: "settings.computer_use.permission_unknown", tone: "warning",
};

const SATISFIED_PERMISSIONS = new Set(["granted", "not_applicable"]);

const ERROR_KEYS = {
  component: "settings.computer_use.failed",
  check: "settings.computer_use.check_failed",
  setup: "settings.computer_use.setup_failed",
} as const;

const EXAMPLE_KEYS = [
  "settings.computer_use.example_calendar",
  "settings.computer_use.example_files",
  "settings.computer_use.example_notes",
] as const;

const MUTED_CLASS_NAME = getUiTypographyClassName({ role: "supporting", tone: "muted" });

function PermissionRow({ name, value }: { name: (typeof PERMISSION_ORDER)[number]; value?: string }) {
  const { t } = useI18n();
  const state = PERMISSION_STATES[value ?? ""] ?? UNKNOWN_PERMISSION_STATE;
  return (
    <li className="flex items-center gap-3 px-4 py-3">
      <div className="min-w-0 flex-1">
        <p className={getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" })}>
          {t(`settings.computer_use.permission_${name}`)}
        </p>
        <p className={cn("mt-0.5", MUTED_CLASS_NAME)}>{t(`settings.computer_use.permission_${name}_hint`)}</p>
      </div>
      <span className={cn("flex shrink-0 items-center gap-1.5", getUiTypographyClassName({ role: "metadata", tone: state.tone }))}>
        <span aria-hidden="true">{state.icon}</span>
        {t(state.labelKey)}
      </span>
    </li>
  );
}

function PermissionSection({ settings }: { settings: ComputerUseSettings }) {
  const { t } = useI18n();
  const permissions = settings.status?.permissions;
  const notRequired = PERMISSION_ORDER.every((key) => permissions?.[key] === "not_applicable");
  const missing = PERMISSION_ORDER.filter((key) => !SATISFIED_PERMISSIONS.has(permissions?.[key] ?? ""));
  const canSetup = !notRequired && missing.length > 0 && getDesktopPlatform() === "macos";
  const meta = notRequired ? null : (
    <span
      className={getUiTypographyClassName({ role: "metadata", tone: missing.length > 0 ? "warning" : "success" })}
      role="status"
    >
      {missing.length > 0
        ? t("settings.computer_use.permissions_missing", { count: missing.length })
        : t("settings.computer_use.permissions_ready")}
    </span>
  );
  const actions = (
    <>
      <UiButton disabled={settings.busy} onClick={() => void settings.onCheckPermissions()} size="xs" variant="text">
        <RefreshCw aria-hidden="true" className="h-3 w-3" />
        {t("settings.computer_use.check_permissions")}
      </UiButton>
      {canSetup && (
        <UiButton disabled={settings.busy} onClick={() => void settings.onOpenPermissions()} size="xs" tone="primary" variant="solid">
          {t("settings.computer_use.open_permissions")}
        </UiButton>
      )}
    </>
  );
  return (
    <SettingsDetailSection actions={actions} meta={meta} title={t("settings.computer_use.permissions_title")}>
      {notRequired ? (
        // Windows 按应用授权，不存在全局开关，不再罗列三个“不适用”。
        <p className={MUTED_CLASS_NAME} role="status">{t("settings.computer_use.permissions_not_required")}</p>
      ) : (
        <ul aria-label={t("settings.computer_use.permissions_title")} className={cn(SETTINGS_CARD_CLASS_NAME, "[&>li+li]:border-t [&>li+li]:border-(--divider-subtle-color)")}>
          {PERMISSION_ORDER.map((key) => <PermissionRow key={key} name={key} value={permissions?.[key]} />)}
        </ul>
      )}
    </SettingsDetailSection>
  );
}

function InfoSection({ settings }: { settings: ComputerUseSettings }) {
  const { t } = useI18n();
  const status = settings.status;
  const platform = getDesktopPlatform();
  const stateKey: TranslationKey = !status?.installed
    ? "settings.computer_use.state_not_installed"
    : status.enabled ? "settings.computer_use.state_enabled" : "settings.computer_use.state_disabled";
  const rows: [TranslationKey, string | undefined][] = [
    ["settings.computer_use.info_status", status ? t(stateKey) : undefined],
    ["settings.computer_use.info_version", status?.version],
    ["settings.computer_use.info_required_version", status?.update_required ? status.required_version : undefined],
    ["settings.computer_use.info_platform", platform === "macos" ? "macOS" : platform === "windows" ? "Windows" : undefined],
    ["settings.computer_use.info_architecture", status?.architecture],
  ];
  return (
    <SettingsDetailInfo
      rows={rows.flatMap(([label, value]) => value ? [[t(label), value] as [string, string]] : [])}
      title={t("settings.computer_use.info_title")}
    />
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
  const subtitleId = useId();
  const status = settings.status;
  const enabled = status?.enabled === true;
  // 进行中的安装或检查直接替换副标题，避免在页面里插入会跳动的状态行。
  const subtitle = settings.busy
    ? t("settings.computer_use.working")
    : status && !status.installed
      ? t("settings.computer_use.install_description")
      : t("settings.computer_use.description");

  return (
    <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
      <WorkspaceContentHeader className="max-sm:hidden" title={t("settings.computer_use.section_title")} />
      <section className={SETTINGS_CONTENT_BODY_CLASS_NAME}>
        <div className={SETTINGS_DETAIL_BODY_CLASS_NAME}>
          <SettingsDetailHeader
            action={(
              <GlassSwitch
                aria-describedby={subtitleId}
                aria-label={t("settings.computer_use.title")}
                checked={enabled}
                disabled={settings.busy || !status}
                onChange={settings.onChange}
                size="sm"
              />
            )}
            icon={<MousePointer2 fill="currentColor" strokeWidth={1.5} />}
            subtitle={subtitle}
            subtitleId={subtitleId}
            title={t("settings.computer_use.section_title")}
          />
          <SettingsDetailExamples
            icon={<MousePointer2 fill="currentColor" strokeWidth={1.5} />}
            items={EXAMPLE_KEYS.map((key) => t(key))}
            label={t("settings.computer_use.examples_label")}
            name={t("settings.computer_use.section_title")}
          />
          <SettingsDetailOverview>{t("settings.computer_use.overview")}</SettingsDetailOverview>
          {status?.update_required && (
            <UiInlineNotice
              action={{ disabled: settings.busy, label: t("settings.computer_use.update"), onClick: () => void settings.onUpdate() }}
              icon={<AlertTriangle />}
              message={t("settings.computer_use.update_required", { version: status.required_version ?? "" })}
              tone="warning"
            />
          )}
          <ComputerUseError settings={settings} />

          {enabled && !status?.update_required && <PermissionSection settings={settings} />}
          <InfoSection settings={settings} />
        </div>
      </section>
    </div>
  );
}
