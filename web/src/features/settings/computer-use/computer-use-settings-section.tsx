/**
 * INPUT: owner Computer Use preference plus host package/sidecar status.
 * OUTPUT: explicit enablement and verified install/update/doctor/start/stop/remove controls.
 * POS: desktop-only settings surface; Browser remains an independent capability.
 */
"use client";

import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, Loader2, MonitorCog, RefreshCw, ShieldAlert } from "lucide-react";

import {
  doctorComputerUseApi,
  getComputerUseStatusApi,
  installComputerUseApi,
  removeComputerUseApi,
  startComputerUseApi,
  stopComputerUseApi,
  type ComputerUseDoctorReport,
  type ComputerUseSidecarState,
  type ComputerUseStatus,
  updateComputerUseApi,
} from "@/lib/api/settings/computer-use-api";
import { getUserPreferencesApi, updateUserPreferencesApi } from "@/lib/api/settings/preferences-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";

type LifecycleAction = "install" | "update" | "doctor" | "start" | "stop" | "remove" | "toggle";

export function ComputerUseSettingsSection() {
  const { t } = useI18n();
  const [status, setStatus] = useState<ComputerUseStatus | null>(null);
  const [doctor, setDoctor] = useState<ComputerUseDoctorReport | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [action, setAction] = useState<LifecycleAction | null>(null);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    try {
      const next = await getComputerUseStatusApi();
      setStatus(next);
      setError("");
    } catch (cause) {
      setError(getErrorMessage(cause, t("settings.computer_use.status_failed")));
    }
  }, [t]);

  useEffect(() => {
    let active = true;
    void Promise.all([getComputerUseStatusApi(), getUserPreferencesApi()])
      .then(([nextStatus, preferences]) => {
        if (!active) return;
        setStatus(nextStatus);
        setEnabled(preferences.computer_use_enabled === true);
      })
      .catch((cause: unknown) => {
        if (active) setError(getErrorMessage(cause, t("settings.computer_use.status_failed")));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [t]);

  useEffect(() => {
    if (!enabled || status?.sidecar.state !== "ready") return;
    const timer = window.setInterval(() => void refresh(), 3_000);
    return () => window.clearInterval(timer);
  }, [enabled, refresh, status?.sidecar.state]);

  const updateEnabled = useCallback(async (nextEnabled: boolean) => {
    const previous = enabled;
    setEnabled(nextEnabled);
    setAction("toggle");
    setError("");
    let persisted: boolean;
    try {
      const preferences = await updateUserPreferencesApi({ computer_use_enabled: nextEnabled });
      persisted = preferences.computer_use_enabled === true;
    } catch (cause) {
      setEnabled(previous);
      setError(getErrorMessage(cause, t("settings.computer_use.toggle_failed")));
      setAction(null);
      return;
    }
    setEnabled(persisted);
    if (!persisted) {
      try {
        await stopComputerUseApi();
      } catch (cause) {
        setError(getErrorMessage(cause, t("settings.computer_use.action_failed")));
      }
    }
    await refresh();
    setAction(null);
  }, [enabled, refresh, t]);

  const run = useCallback(async (nextAction: Exclude<LifecycleAction, "toggle">) => {
    if (nextAction === "remove" && !window.confirm(t("settings.computer_use.remove_confirm"))) return;
    setAction(nextAction);
    setError("");
    try {
      switch (nextAction) {
        case "install": await installComputerUseApi(); break;
        case "update": await updateComputerUseApi(); break;
        case "doctor": setDoctor(await doctorComputerUseApi()); break;
        case "start": await startComputerUseApi(); break;
        case "stop": await stopComputerUseApi(); break;
        case "remove": await removeComputerUseApi(); setDoctor(null); break;
      }
      await refresh();
    } catch (cause) {
      setError(getErrorMessage(cause, t("settings.computer_use.action_failed")));
    } finally {
      setAction(null);
    }
  }, [refresh, t]);

  const busy = action !== null;
  const packageStatus = status?.package;
  const sidecar = status?.sidecar;
  const ready = sidecar?.state === "ready";
  const sidecarStateLabels: Record<ComputerUseSidecarState, string> = {
    stopped: t("settings.computer_use.state_stopped"),
    starting: t("settings.computer_use.state_starting"),
    ready: t("settings.computer_use.state_ready"),
    stopping: t("settings.computer_use.state_stopping"),
    failed: t("settings.computer_use.state_failed"),
  };

  return (
    <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
      <WorkspaceContentHeader
        className="max-sm:hidden"
        description={t("settings.computer_use.section_description")}
        title={t("settings.computer_use.section_title")}
      />

      <section className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
        <div className="flex items-start justify-between gap-4 px-4 py-4">
          <div className="flex min-w-0 items-start gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
              <MonitorCog className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-base font-semibold text-(--text-strong)">{t("settings.computer_use.enable_title")}</h2>
              <p className="mt-1 max-w-[720px] text-compact leading-5 text-(--text-soft)">{t("settings.computer_use.enable_description")}</p>
            </div>
          </div>
          <GlassSwitch
            aria-label={t("settings.computer_use.enable_title")}
            checked={enabled}
            disabled={loading || busy || packageStatus?.available === false}
            onChange={(value) => void updateEnabled(value)}
            size="sm"
          />
        </div>

        <div className="border-t border-(--divider-subtle-color) px-4 py-4">
          <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-compact">
            <StatusItem label={t("settings.computer_use.package_label")} value={packageStatus?.installed ? `v${packageStatus.version ?? "?"}` : t("settings.computer_use.not_installed")} />
            <StatusItem label={t("settings.computer_use.platform_label")} value={packageStatus ? `${packageStatus.platform} / ${packageStatus.architecture}` : "—"} />
            <StatusItem label={t("settings.computer_use.runtime_label")} value={sidecar ? sidecarStateLabels[sidecar.state] ?? "—" : "—"} />
          </div>

          <div className="mt-4 flex flex-wrap gap-2">
            {packageStatus?.can_install ? <ActionButton disabled={busy} loading={action === "install"} label={t("settings.computer_use.install")} onClick={() => void run("install")} /> : null}
            {packageStatus?.can_update ? <ActionButton disabled={busy} loading={action === "update"} label={t("settings.computer_use.update")} onClick={() => void run("update")} /> : null}
            {packageStatus?.installed && enabled && !ready ? <ActionButton disabled={busy} loading={action === "start"} label={t("settings.computer_use.start")} onClick={() => void run("start")} /> : null}
            {ready || sidecar?.state === "starting" ? <ActionButton disabled={busy} loading={action === "stop"} label={t("settings.computer_use.stop")} onClick={() => void run("stop")} /> : null}
            {packageStatus?.installed ? <ActionButton disabled={busy} loading={action === "doctor"} label={t("settings.computer_use.doctor")} onClick={() => void run("doctor")} /> : null}
            {packageStatus?.can_remove ? <UiButton disabled={busy} onClick={() => void run("remove")} size="sm" tone="danger" variant="text">{t("settings.computer_use.remove")}</UiButton> : null}
            <UiButton disabled={busy} onClick={() => void refresh()} size="sm" variant="text"><RefreshCw className="h-3.5 w-3.5" />{t("settings.computer_use.refresh")}</UiButton>
          </div>

          {packageStatus?.message ? <p className="mt-3 text-xs text-(--text-soft)">{packageStatus.message}</p> : null}
          {sidecar?.last_error ? <p className="mt-3 text-xs text-(--destructive)" role="alert">{sidecar.last_error}</p> : null}
          {error ? <p className="mt-3 text-xs text-(--destructive)" role="alert">{error}</p> : null}
        </div>
      </section>

      <section className="mt-7 rounded-[12px] border border-(--divider-subtle-color) px-4 py-4">
        <div className="flex items-start gap-2.5">
          {doctor?.healthy ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-(--success)" /> : <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-(--warning)" />}
          <div>
            <h2 className="text-sm font-semibold text-(--text-strong)">{t("settings.computer_use.alpha_title")}</h2>
            <p className="mt-1 text-xs leading-5 text-(--text-soft)">{t("settings.computer_use.alpha_description")}</p>
            {doctor ? <p className="mt-2 text-xs text-(--text-muted)">{doctor.healthy ? t("settings.computer_use.doctor_healthy") : doctor.message ?? t("settings.computer_use.doctor_unhealthy")}</p> : null}
          </div>
        </div>
      </section>
    </div>
  );
}

function StatusItem({ label, value }: { label: string; value: string }) {
  return <span><span className="text-(--text-soft)">{label}</span><span className="ml-1.5 font-medium text-(--text-strong)">{value}</span></span>;
}

function ActionButton({ disabled, loading, label, onClick }: { disabled: boolean; loading: boolean; label: string; onClick: () => void }) {
  return <UiButton disabled={disabled} onClick={onClick} size="sm" variant="surface">{loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}{label}</UiButton>;
}
