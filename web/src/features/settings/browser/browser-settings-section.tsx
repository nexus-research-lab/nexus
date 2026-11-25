/**
 * INPUT: 桌面浏览器扩展状态、安装命令和 CDP 偏好。
 * OUTPUT: 复用设置详情 Pattern 的 Browser 页：身份头与安装动作、示例、连接与恢复、CDP 开关和扩展信息。
 * POS: 设置目录的 Browser 分区，移动端页面身份由应用栏承载。
 */
"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import {
  AppWindow,
  CheckCircle2,
  FolderOpen,
  Loader2,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";

import { getBrowserExtensionStatusApi, type BrowserExtensionStatus } from "@/lib/api/settings/browser-api";
import {
  startDesktopBrowserExtensionSetup,
  type DesktopBrowserExtensionSetupResult,
} from "@/lib/desktop-bridge/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { PreferencesReliabilityNotice } from "../general/components/preferences-reliability-notice";
import { useUserPreferences } from "../general/use-user-preferences";
import {
  SETTINGS_DETAIL_BODY_CLASS_NAME,
  SettingsDetailExamples,
  SettingsDetailHeader,
  SettingsDetailInfo,
  SettingsDetailOverview,
  SettingsDetailSection,
} from "../shared/settings-detail-page";
import { SETTINGS_CARD_CLASS_NAME, SETTINGS_CONTENT_BODY_CLASS_NAME } from "../shared/settings-panel-ui";

const STATUS_POLL_INTERVAL_MS = 2_000;

const EXAMPLE_KEYS = [
  "settings.browser.example_notifications",
  "settings.browser.example_export",
  "settings.browser.example_compare",
] as const;

const INSTALL_STEPS: ReadonlyArray<{
  descriptionKey: TranslationKey;
  titleKey: TranslationKey;
}> = [
  {
    titleKey: "settings.browser.install_step_developer_title",
    descriptionKey: "settings.browser.install_step_developer_description",
  },
  {
    titleKey: "settings.browser.install_step_load_title",
    descriptionKey: "settings.browser.install_step_load_description",
  },
  {
    titleKey: "settings.browser.install_step_wait_title",
    descriptionKey: "settings.browser.install_step_wait_description",
  },
];

export function BrowserSettingsSection() {
  const { t } = useI18n();
  const cdpDescriptionId = useId();
  const cdpRiskId = useId();
  const [status, setStatus] = useState<BrowserExtensionStatus | null>(null);
  const [statusError, setStatusError] = useState(false);
  const [statusRefresh, setStatusRefresh] = useState(0);
  const [setup, setSetup] = useState<DesktopBrowserExtensionSetupResult | null>(null);
  const [openingSetup, setOpeningSetup] = useState(false);
  const setupPendingRef = useRef(false);
  const [setupError, setSetupError] = useState(false);
  const preferences = useUserPreferences();
  const cdpEnabled = preferences.preferences.browser_cdp_enabled === true;

  useEffect(() => {
    let active = true;
    let loading = false;
    const load = async () => {
      if (!active || loading) return;
      loading = true;
      try {
        const next = await getBrowserExtensionStatusApi();
        if (!active) return;
        setStatus(next);
        setStatusError(false);
      } catch {
        if (!active) return;
        setStatusError(true);
      } finally {
        loading = false;
      }
    };
    void load();
    const interval = window.setInterval(() => void load(), STATUS_POLL_INTERVAL_MS);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [statusRefresh]);

  const openSetup = useCallback(async () => {
    if (setupPendingRef.current) return;
    setupPendingRef.current = true;
    setOpeningSetup(true);
    setSetupError(false);
    try {
      setSetup(await startDesktopBrowserExtensionSetup());
    } catch {
      setSetupError(true);
    } finally {
      setupPendingRef.current = false;
      setOpeningSetup(false);
    }
  }, []);

  const updateCDPAccess = useCallback((enabled: boolean) => {
    preferences.updatePreferences((current) => ({
      ...current,
      browser_cdp_enabled: enabled,
    }));
  }, [preferences]);

  const connected = status?.connected === true;
  const incompatible = status?.connection_state === "incompatible";
  const browserName = status?.browser_name ?? setup?.browser_name ?? t("settings.browser.chromium_title");
  const statusLabel = status === null && !statusError
    ? t("settings.browser.status_checking")
    : connected
      ? t("settings.browser.status_connected")
      : incompatible
        ? t("settings.browser.status_incompatible")
        : t("settings.browser.status_disconnected");
  const statusTone = connected
    ? "success"
    : incompatible
      ? "warning"
      : "idle";

  return (
    <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
      <WorkspaceContentHeader
        className="max-sm:hidden"
        title={t("settings.browser.section_title")}
      />

      <section className={SETTINGS_CONTENT_BODY_CLASS_NAME}>
        <div className={SETTINGS_DETAIL_BODY_CLASS_NAME}>
          <SettingsDetailHeader
            action={(
              <UiButton
                aria-busy={openingSetup}
                disabled={openingSetup}
                onClick={() => void openSetup()}
                size="sm"
                tone={connected ? "default" : "primary"}
                variant={connected ? "surface" : "solid"}
              >
                {openingSetup ? (
                  <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
                ) : (
                  <FolderOpen className="h-3.5 w-3.5" />
                )}
                {openingSetup
                  ? t("settings.browser.install_opening")
                  : incompatible
                    ? t("settings.browser.install_update")
                    : connected || setup !== null
                    ? t("settings.browser.install_reopen")
                    : t("settings.browser.install_action")}
              </UiButton>
            )}
            icon={<AppWindow />}
            subtitle={(
              <span aria-live="polite" className="flex flex-wrap items-center gap-2">
                <span>{browserName}</span>
                <UiBadge showDot size="sm" tone={statusTone}>{statusLabel}</UiBadge>
              </span>
            )}
            title={t("settings.browser.section_title")}
          />
          <SettingsDetailExamples
            icon={<AppWindow />}
            items={EXAMPLE_KEYS.map((key) => t(key))}
            label={t("settings.browser.examples_label")}
            name={t("settings.browser.section_title")}
          />
          <SettingsDetailOverview>{t("settings.browser.section_description")}</SettingsDetailOverview>

          <SettingsDetailSection title={t("settings.browser.connection_title")}>
            <div className={cn(SETTINGS_CARD_CLASS_NAME, "px-4 py-3")}>
              {connected ? (
                <div className={cn(
                  "flex items-center gap-2",
                  getUiTypographyClassName({ role: "metadata", tone: "success", weight: "medium" }),
                )}>
                  <CheckCircle2 className="h-4 w-4 shrink-0" />
                  <span>{t("settings.browser.install_success", { browser: browserName })}</span>
                </div>
              ) : incompatible ? (
                <UiResourceState
                  className="p-0"
                  impact={t("settings.browser.incompatible_impact")}
                  nextStep={t("settings.browser.incompatible_next_step")}
                  size="sm"
                  state="error"
                  title={t("settings.browser.incompatible_title")}
                  tone="warning"
                  variant="plain"
                />
              ) : setup !== null ? (
                <div>
                  <p
                    className={getUiTypographyClassName({
                      role: "metadata",
                      tone: "strong",
                      weight: "semibold",
                    })}
                  >
                    {t("settings.browser.install_guide_title", { browser: browserName })}
                  </p>
                  <ol className="mt-3 grid gap-4 md:grid-cols-3">
                    {INSTALL_STEPS.map((step, index) => (
                      <li className="flex items-start gap-2.5" key={step.titleKey}>
                        <span className={cn(
                          "flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)]",
                          getUiTypographyClassName({ role: "caption", tone: "brand", weight: "semibold" }),
                        )}>
                          {index + 1}
                        </span>
                        <div>
                          <p
                            className={getUiTypographyClassName({
                              role: "metadata",
                              tone: "strong",
                              weight: "semibold",
                            })}
                          >
                            {t(step.titleKey)}
                          </p>
                          <p className={cn("mt-1", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
                            {t(step.descriptionKey, { browser: browserName })}
                          </p>
                        </div>
                      </li>
                    ))}
                  </ol>
                </div>
              ) : (
                <p className={getUiTypographyClassName({ role: "metadata", tone: "soft" })}>
                  {t("settings.browser.install_hint")}
                </p>
              )}

              {setupError || statusError ? (
                <UiResourceState
                  className="mt-3 p-0"
                  impact={t(setupError
                    ? "settings.browser.install_failed_impact"
                    : "settings.browser.status_failed_impact")}
                  primaryAction={{
                    icon: <RefreshCw className="h-3 w-3" />,
                    label: setupError
                      ? t("settings.browser.install_action")
                      : t("settings.browser.refresh"),
                    onClick: () => {
                      if (setupError) {
                        void openSetup();
                        return;
                      }
                      setStatusRefresh((value) => value + 1);
                    },
                  }}
                  size="sm"
                  state="error"
                  title={t(setupError
                    ? "settings.browser.install_failed"
                    : "settings.browser.status_failed")}
                  variant="plain"
                />
              ) : null}
            </div>
          </SettingsDetailSection>

          <SettingsDetailSection title={t("settings.browser.developer_title")}>
            <PreferencesReliabilityNotice
              feedback={preferences.feedback}
              recovery={preferences.recovery}
            />
            <div className={cn(SETTINGS_CARD_CLASS_NAME, "px-4 py-4")}>
              <div className="flex items-start justify-between gap-4">
                <div className="flex min-w-0 items-start gap-3">
                  <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />
                  <div className="min-w-0 break-words">
                    <p
                      id={cdpRiskId}
                      className={getUiTypographyClassName({
                        role: "caption",
                        tone: "warning",
                        weight: "semibold",
                      })}
                    >
                      {t("settings.browser.cdp_risk")}
                    </p>
                    <h3 className={cn("mt-1", getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }))}>
                      {t("settings.browser.cdp_title")}
                    </h3>
                    <p id={cdpDescriptionId} className={cn(
                      "mt-1 max-w-[720px]",
                      getUiTypographyClassName({ role: "supporting", tone: "muted" }),
                    )}>
                      {t("settings.browser.cdp_description")}
                    </p>
                  </div>
                </div>
                <GlassSwitch
                  aria-describedby={`${cdpRiskId} ${cdpDescriptionId}`}
                  aria-label={t("settings.browser.cdp_toggle")}
                  checked={cdpEnabled}
                  disabled={preferences.loading || preferences.saving || !preferences.writable}
                  onChange={updateCDPAccess}
                  size="sm"
                />
              </div>
            </div>
          </SettingsDetailSection>

          <SettingsDetailInfo
            rows={[
              [t("settings.browser.info_status"), statusLabel],
              [t("settings.browser.info_browser"), browserName],
              ...(status?.extension_version ? [[t("settings.browser.info_version"), status.extension_version] as [string, string]] : []),
            ]}
            title={t("settings.browser.info_title")}
          />
        </div>
      </section>
    </div>
  );
}
