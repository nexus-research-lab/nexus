"use client";

import { useCallback, useEffect, useState } from "react";
import {
  AppWindow,
  CheckCircle2,
  FolderOpen,
  Loader2,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";

import { getBrowserExtensionStatusApi, type BrowserExtensionStatus } from "@/lib/api/settings/browser-api";
import { getUserPreferencesApi, updateUserPreferencesApi } from "@/lib/api/settings/preferences-api";
import {
  startDesktopBrowserExtensionSetup,
  type DesktopBrowserExtensionSetupResult,
} from "@/lib/desktop-bridge/desktop-bridge";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";

const STATUS_POLL_INTERVAL_MS = 2_000;

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
  const [status, setStatus] = useState<BrowserExtensionStatus | null>(null);
  const [statusError, setStatusError] = useState("");
  const [statusRefresh, setStatusRefresh] = useState(0);
  const [setup, setSetup] = useState<DesktopBrowserExtensionSetupResult | null>(null);
  const [openingSetup, setOpeningSetup] = useState(false);
  const [setupError, setSetupError] = useState("");
  const [cdpEnabled, setCDPEnabled] = useState(false);
  const [cdpLoading, setCDPLoading] = useState(true);
  const [cdpSaving, setCDPSaving] = useState(false);
  const [cdpError, setCDPError] = useState("");

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const next = await getBrowserExtensionStatusApi();
        if (!active) return;
        setStatus(next);
        setStatusError("");
      } catch (error) {
        if (!active) return;
        setStatusError(getErrorMessage(error, t("settings.browser.status_failed")));
      }
    };
    void load();
    const interval = window.setInterval(() => void load(), STATUS_POLL_INTERVAL_MS);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [statusRefresh, t]);

  useEffect(() => {
    let active = true;
    void getUserPreferencesApi()
      .then((preferences) => {
        if (active) setCDPEnabled(preferences.browser_cdp_enabled === true);
      })
      .catch((error: unknown) => {
        if (active) {
          setCDPError(getErrorMessage(error, t("settings.browser.cdp_load_failed")));
        }
      })
      .finally(() => {
        if (active) setCDPLoading(false);
      });
    return () => {
      active = false;
    };
  }, [t]);

  const openSetup = useCallback(async () => {
    setOpeningSetup(true);
    setSetupError("");
    try {
      setSetup(await startDesktopBrowserExtensionSetup());
    } catch (error) {
      setSetupError(getErrorMessage(error, t("settings.browser.install_failed")));
    } finally {
      setOpeningSetup(false);
    }
  }, [t]);

  const updateCDPAccess = useCallback(async (enabled: boolean) => {
    const previous = cdpEnabled;
    setCDPEnabled(enabled);
    setCDPSaving(true);
    setCDPError("");
    try {
      const preferences = await updateUserPreferencesApi({ browser_cdp_enabled: enabled });
      setCDPEnabled(preferences.browser_cdp_enabled === true);
    } catch (error) {
      setCDPEnabled(previous);
      setCDPError(getErrorMessage(error, t("settings.browser.cdp_save_failed")));
    } finally {
      setCDPSaving(false);
    }
  }, [cdpEnabled, t]);

  const connected = status?.connected === true;
  const incompatible = status?.connection_state === "incompatible";
  const browserName = status?.browser_name ?? setup?.browser_name ?? t("settings.browser.chromium_title");
  const statusLabel = status === null && statusError === ""
    ? t("settings.browser.status_checking")
    : connected
      ? t("settings.browser.status_connected")
      : incompatible
        ? t("settings.browser.status_incompatible")
        : t("settings.browser.status_disconnected");
  const statusColor = connected
    ? "text-(--success)"
    : incompatible
      ? "text-(--warning)"
      : "text-(--text-soft)";
  const statusDot = connected
    ? "bg-(--success)"
    : incompatible
      ? "bg-(--warning)"
      : "bg-(--icon-muted)";

  return (
    <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
      <WorkspaceContentHeader
        description={t("settings.browser.section_description")}
        title={t("settings.browser.section_title")}
      />

      <section className="space-y-2.5">
        <div className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
          <div className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                <AppWindow className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
                  {browserName}
                </h2>
                <div aria-live="polite" className="mt-1 flex flex-wrap items-center gap-2 text-compact">
                  <span className={statusColor}>
                    <span
                      aria-hidden="true"
                      className={`mr-1.5 inline-block h-2 w-2 rounded-full ${statusDot}`}
                    />
                    {statusLabel}
                  </span>
                  {connected && status?.extension_version ? (
                    <span className="text-(--text-soft)">
                      {t("settings.browser.status_version", { version: status.extension_version })}
                    </span>
                  ) : null}
                </div>
              </div>
            </div>

            <UiButton
              disabled={openingSetup}
              onClick={() => void openSetup()}
              size="sm"
              tone={connected ? "default" : "primary"}
              variant={connected ? "surface" : "solid"}
            >
              {openingSetup ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <FolderOpen className="h-3.5 w-3.5" />}
              {openingSetup
                ? t("settings.browser.install_opening")
                : incompatible
                  ? t("settings.browser.install_update")
                  : connected || setup !== null
                  ? t("settings.browser.install_reopen")
                  : t("settings.browser.install_action")}
            </UiButton>
          </div>

          <div className="border-t border-(--divider-subtle-color) px-4 py-3">
            {connected ? (
              <div className="flex items-center gap-2 text-compact text-(--success)">
                <CheckCircle2 className="h-4 w-4 shrink-0" />
                <span>{t("settings.browser.install_success", { browser: browserName })}</span>
              </div>
            ) : incompatible ? (
              <div className="flex items-start gap-2.5 text-(--warning)" role="alert">
                <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <p className="text-compact font-semibold">
                    {t("settings.browser.incompatible_title")}
                  </p>
                  <p className="mt-1 text-xs leading-5 text-(--text-soft)">
                    {t("settings.browser.incompatible_description", {
                      current: status?.observed_protocol_version ?? "?",
                      expected: status?.protocol_version ?? "?",
                      version: status?.observed_extension_version ?? "?",
                    })}
                  </p>
                </div>
              </div>
            ) : setup !== null ? (
              <div>
                <p className="text-compact font-semibold text-(--text-strong)">
                  {t("settings.browser.install_guide_title", { browser: browserName })}
                </p>
                <ol className="mt-3 grid gap-4 md:grid-cols-3">
                  {INSTALL_STEPS.map((step, index) => (
                    <li className="flex items-start gap-2.5" key={step.titleKey}>
                      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-[11px] font-semibold text-primary">
                        {index + 1}
                      </span>
                      <div>
                        <p className="text-compact font-semibold text-(--text-strong)">{t(step.titleKey)}</p>
                        <p className="mt-1 text-xs leading-5 text-(--text-soft)">
                          {t(step.descriptionKey, { browser: browserName })}
                        </p>
                      </div>
                    </li>
                  ))}
                </ol>
              </div>
            ) : (
              <p className="text-compact leading-5 text-(--text-soft)">
                {t("settings.browser.install_hint")}
              </p>
            )}

            {setupError || statusError ? (
              <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-(--destructive)" role="alert">
                <span>{setupError || statusError}</span>
                <UiButton
                  onClick={() => {
                    if (setupError) {
                      void openSetup();
                      return;
                    }
                    setStatusRefresh((value) => value + 1);
                  }}
                  size="xs"
                  variant="text"
                >
                  <RefreshCw className="h-3 w-3" />
                  {setupError
                    ? t("settings.browser.install_action")
                    : t("settings.browser.refresh")}
                </UiButton>
              </div>
            ) : null}
          </div>
        </div>
      </section>

      <section className="mt-7 space-y-2.5">
        <h2 className="px-1 text-md font-semibold tracking-tight text-(--text-strong)">
          {t("settings.browser.developer_title")}
        </h2>
        <div className="rounded-[12px] border border-(--divider-subtle-color) bg-transparent px-4 py-4">
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 items-start gap-3">
              <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />
              <div>
                <p className="text-xs font-semibold text-(--warning)">
                  {t("settings.browser.cdp_risk")}
                </p>
                <h3 className="mt-1 text-[14px] font-semibold tracking-tight text-(--text-strong)">
                  {t("settings.browser.cdp_title")}
                </h3>
                <p className="mt-1 max-w-[720px] text-compact leading-5 text-(--text-soft)">
                  {t("settings.browser.cdp_description")}
                </p>
                {cdpError ? <p className="mt-2 text-xs text-(--destructive)" role="alert">{cdpError}</p> : null}
              </div>
            </div>
            <GlassSwitch
              aria-label={t("settings.browser.cdp_toggle")}
              checked={cdpEnabled}
              disabled={cdpLoading || cdpSaving}
              onChange={(enabled) => void updateCDPAccess(enabled)}
              size="sm"
            />
          </div>
        </div>
      </section>
    </div>
  );
}
