"use client";

import { useCallback, useEffect, useState } from "react";
import {
  CheckCircle2,
  Chrome,
  FolderOpen,
  Loader2,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";

import { getBrowserExtensionStatusApi, type BrowserExtensionStatus } from "@/lib/api/settings/webbridge-api";
import { getUserPreferencesApi, updateUserPreferencesApi } from "@/lib/api/settings/preferences-api";
import { startDesktopBrowserExtensionSetup } from "@/lib/desktop-bridge/desktop-bridge";
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
    titleKey: "settings.computer_use.install_step_developer_title",
    descriptionKey: "settings.computer_use.install_step_developer_description",
  },
  {
    titleKey: "settings.computer_use.install_step_load_title",
    descriptionKey: "settings.computer_use.install_step_load_description",
  },
  {
    titleKey: "settings.computer_use.install_step_wait_title",
    descriptionKey: "settings.computer_use.install_step_wait_description",
  },
];

export function ComputerUseSettingsSection() {
  const { t } = useI18n();
  const [status, setStatus] = useState<BrowserExtensionStatus | null>(null);
  const [statusError, setStatusError] = useState("");
  const [statusRefresh, setStatusRefresh] = useState(0);
  const [setupStarted, setSetupStarted] = useState(false);
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
        setStatusError(getErrorMessage(error, t("settings.computer_use.status_failed")));
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
          setCDPError(getErrorMessage(error, t("settings.computer_use.cdp_load_failed")));
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
      await startDesktopBrowserExtensionSetup();
      setSetupStarted(true);
    } catch (error) {
      setSetupError(getErrorMessage(error, t("settings.computer_use.install_failed")));
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
      setCDPError(getErrorMessage(error, t("settings.computer_use.cdp_save_failed")));
    } finally {
      setCDPSaving(false);
    }
  }, [cdpEnabled, t]);

  const connected = status?.connected === true;
  const statusLabel = status === null && statusError === ""
    ? t("settings.computer_use.status_checking")
    : connected
      ? t("settings.computer_use.status_connected")
      : t("settings.computer_use.status_disconnected");

  return (
    <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
      <WorkspaceContentHeader
        description={t("settings.computer_use.section_description")}
        title={t("settings.computer_use.section_title")}
      />

      <section className="space-y-2.5">
        <div className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
          <div className="grid gap-4 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                <Chrome className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
                  {t("settings.computer_use.chrome_title")}
                </h2>
                <div aria-live="polite" className="mt-1 flex flex-wrap items-center gap-2 text-compact">
                  <span className={connected ? "text-(--success)" : "text-(--text-soft)"}>
                    <span
                      aria-hidden="true"
                      className={`mr-1.5 inline-block h-2 w-2 rounded-full ${connected ? "bg-(--success)" : "bg-(--icon-muted)"}`}
                    />
                    {statusLabel}
                  </span>
                  {connected && status?.extension_version ? (
                    <span className="text-(--text-soft)">
                      {t("settings.computer_use.status_version", { version: status.extension_version })}
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
                ? t("settings.computer_use.install_opening")
                : connected || setupStarted
                  ? t("settings.computer_use.install_reopen")
                  : t("settings.computer_use.install_action")}
            </UiButton>
          </div>

          <div className="border-t border-(--divider-subtle-color) px-4 py-3">
            {connected ? (
              <div className="flex items-center gap-2 text-compact text-(--success)">
                <CheckCircle2 className="h-4 w-4 shrink-0" />
                <span>{t("settings.computer_use.install_success")}</span>
              </div>
            ) : setupStarted ? (
              <div>
                <p className="text-compact font-semibold text-(--text-strong)">
                  {t("settings.computer_use.install_guide_title")}
                </p>
                <ol className="mt-3 grid gap-4 md:grid-cols-3">
                  {INSTALL_STEPS.map((step, index) => (
                    <li className="flex items-start gap-2.5" key={step.titleKey}>
                      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-[11px] font-semibold text-primary">
                        {index + 1}
                      </span>
                      <div>
                        <p className="text-compact font-semibold text-(--text-strong)">{t(step.titleKey)}</p>
                        <p className="mt-1 text-xs leading-5 text-(--text-soft)">{t(step.descriptionKey)}</p>
                      </div>
                    </li>
                  ))}
                </ol>
              </div>
            ) : (
              <p className="text-compact leading-5 text-(--text-soft)">
                {t("settings.computer_use.install_hint")}
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
                    ? t("settings.computer_use.install_action")
                    : t("settings.computer_use.refresh")}
                </UiButton>
              </div>
            ) : null}
          </div>
        </div>
      </section>

      <section className="mt-7 space-y-2.5">
        <h2 className="px-1 text-md font-semibold tracking-tight text-(--text-strong)">
          {t("settings.computer_use.developer_title")}
        </h2>
        <div className="rounded-[12px] border border-(--divider-subtle-color) bg-transparent px-4 py-4">
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 items-start gap-3">
              <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />
              <div>
                <p className="text-xs font-semibold text-(--warning)">
                  {t("settings.computer_use.cdp_risk")}
                </p>
                <h3 className="mt-1 text-[14px] font-semibold tracking-tight text-(--text-strong)">
                  {t("settings.computer_use.cdp_title")}
                </h3>
                <p className="mt-1 max-w-[720px] text-compact leading-5 text-(--text-soft)">
                  {t("settings.computer_use.cdp_description")}
                </p>
                {cdpError ? <p className="mt-2 text-xs text-(--destructive)" role="alert">{cdpError}</p> : null}
              </div>
            </div>
            <GlassSwitch
              aria-label={t("settings.computer_use.cdp_toggle")}
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
