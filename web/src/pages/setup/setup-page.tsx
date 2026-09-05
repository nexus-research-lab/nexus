// INPUT: 认证启动状态、一次性 Setup capability、owner 草稿与初始化命令结果。
// OUTPUT: 首次 Deployment owner 初始化表单、共享失败提示及成功后的 Launcher 导航。
// POS: Setup 页面业务入口；不持久化 capability，也不在结果未知时自动重放创建请求。
"use client";

import { ArrowRight, CheckCircle2, KeyRound, ServerCog } from "lucide-react";
import { useMemo, useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { setupControlOwnerApi } from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { AppLoadingState } from "@/shared/ui/layout/app-loading-screen";

interface SetupDraft {
  setupToken: string;
  deploymentName: string;
  username: string;
  displayName: string;
  password: string;
  confirmPassword: string;
}

const INITIAL_DRAFT: SetupDraft = {
  setupToken: "",
  deploymentName: "Nexus",
  username: "admin",
  displayName: "Admin",
  password: "",
  confirmPassword: "",
};

export function SetupPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { isBootstrapped, refreshStatus, status } = useAuth();
  const [draft, setDraft] = useState(INITIAL_DRAFT);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [failure, setFailure] = useState(false);
  const validationKey = useMemo(() => validateSetupDraft(draft), [draft]);

  if (!isBootstrapped) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-(--secondary)">
        <AppLoadingState message={t("setup.loading")} />
      </main>
    );
  }
  if (status && !status.setup_required) {
    return (
      <Navigate
        replace
        to={status.authenticated ? APP_ROUTE_PATHS.launcher : APP_ROUTE_PATHS.login}
      />
    );
  }

  if (status?.setup_enabled === false) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-(--secondary) px-6 py-10 text-foreground">
        <section className="surface-panel surface-radius-xl w-full max-w-[480px] border px-8 py-9 text-center">
          <ServerCog className="mx-auto h-10 w-10 text-(--brand)" />
          <h1 className="mt-5 text-xl font-semibold text-(--text-strong)">
            {t("setup.disabled_title")}
          </h1>
          <p className="mt-3 text-sm leading-6 text-(--text-muted)">
            {t("setup.disabled_description")}
          </p>
        </section>
      </main>
    );
  }

  const setField = (field: keyof SetupDraft, value: string) => {
    setDraft((current) => ({ ...current, [field]: value }));
    setFailure(false);
  };
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (validationKey) {
      return;
    }
    setIsSubmitting(true);
    setFailure(false);
    try {
      await setupControlOwnerApi({
        setupToken: draft.setupToken,
        deploymentName: draft.deploymentName,
        username: draft.username,
        displayName: draft.displayName,
        password: draft.password,
      });
      await refreshStatus();
      navigate(APP_ROUTE_PATHS.launcher, { replace: true });
    } catch {
      setFailure(true);
      void refreshStatus()
        .then((nextStatus) => {
          if (!nextStatus.setup_required) {
            navigate(
              nextStatus.authenticated ? APP_ROUTE_PATHS.launcher : APP_ROUTE_PATHS.login,
              { replace: true },
            );
          }
        })
        .catch(() => undefined);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <main className="relative min-h-screen overflow-hidden bg-(--secondary) px-5 py-8 text-foreground sm:px-8 lg:px-10">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-(--secondary) bg-[linear-gradient(rgba(255,255,255,0.18),rgba(255,255,255,0.18)),linear-gradient(90deg,rgba(255,255,255,0.46)_1px,transparent_1px),linear-gradient(60deg,rgba(255,255,255,0.42)_1px,transparent_1px),linear-gradient(120deg,rgba(255,255,255,0.42)_1px,transparent_1px)] bg-[length:100%_100%,160px_138px,160px_138px,160px_138px]"
      />
      <div className="relative mx-auto grid min-h-[calc(100vh-4rem)] w-full max-w-[1180px] grid-cols-1 items-center gap-8 lg:grid-cols-[minmax(0,0.9fr)_minmax(420px,500px)] lg:gap-16">
        <section className="min-w-0 py-6">
          <Link
            aria-label={t("setup.back_home")}
            className="inline-flex items-center gap-3 text-(--text-strong) no-underline"
            to={APP_ROUTE_PATHS.root}
          >
            <img alt="" className="h-10 w-10 object-contain" src="/logo.webp" />
            <span className="text-xl font-semibold leading-none">NEXUS</span>
          </Link>
          <div className="mt-14 max-w-[560px] lg:mt-20">
            <p className="text-sm font-semibold text-(--text-soft)">{t("setup.eyebrow")}</p>
            <h1 className="mt-4 text-[42px] font-semibold leading-[1.02] text-(--text-strong) sm:text-[58px]">
              {t("setup.headline")}
            </h1>
            <p className="mt-6 max-w-[500px] text-md leading-8 text-(--text-muted)">
              {t("setup.description")}
            </p>
          </div>
          <div className="mt-10 grid max-w-[560px] gap-3 border-t border-(--material-input-border) pt-5 text-sm text-(--text-muted) sm:grid-cols-2">
            <p className="flex gap-2"><CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />{t("setup.fact_identity")}</p>
            <p className="flex gap-2"><CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />{t("setup.fact_data")}</p>
          </div>
        </section>

        <section className="relative w-full rounded-[12px] border border-[color:color-mix(in_srgb,var(--card)_97%,transparent)] bg-[color:color-mix(in_srgb,var(--card)_88%,transparent)] p-6 shadow-(--modal-dialog-surface-shadow) backdrop-blur-xl sm:p-7">
          <div className="flex items-start justify-between gap-5">
            <div>
              <div className="inline-flex items-center gap-2 text-xs font-semibold text-(--text-muted)">
                <ServerCog className="h-4 w-4 text-(--brand)" />
                {t("setup.form_eyebrow")}
              </div>
              <h2 className="mt-3 text-xl font-semibold text-(--text-strong)">{t("setup.form_title")}</h2>
            </div>
            <KeyRound className="h-6 w-6 text-(--icon-muted)" />
          </div>

          <form className="mt-6 space-y-4" onSubmit={submit}>
            <UiField htmlFor="setup-capability" label={t("setup.capability")} required>
              <UiInput
                autoComplete="off"
                id="setup-capability"
                minLength={32}
                onChange={(event) => setField("setupToken", event.target.value)}
                required
                type="password"
                value={draft.setupToken}
                variant="surface"
              />
            </UiField>
            <div className="grid gap-4 sm:grid-cols-2">
              <UiField htmlFor="setup-deployment" label={t("setup.deployment_name")} required>
                <UiInput id="setup-deployment" maxLength={128} onChange={(event) => setField("deploymentName", event.target.value)} required value={draft.deploymentName} variant="surface" />
              </UiField>
              <UiField htmlFor="setup-username" label={t("login.username")} required>
                <UiInput autoComplete="username" id="setup-username" maxLength={64} minLength={3} onChange={(event) => setField("username", event.target.value)} pattern="[a-z0-9._-]+" required value={draft.username} variant="surface" />
              </UiField>
            </div>
            <UiField htmlFor="setup-display-name" label={t("setup.display_name")} required>
              <UiInput id="setup-display-name" maxLength={128} onChange={(event) => setField("displayName", event.target.value)} required value={draft.displayName} variant="surface" />
            </UiField>
            <div className="grid gap-4 sm:grid-cols-2">
              <UiField htmlFor="setup-password" label={t("setup.password")} required>
                <UiInput autoComplete="new-password" id="setup-password" minLength={8} onChange={(event) => setField("password", event.target.value)} required type="password" value={draft.password} variant="surface" />
              </UiField>
              <UiField htmlFor="setup-confirm-password" label={t("setup.confirm_password")} required>
                <UiInput autoComplete="new-password" id="setup-confirm-password" minLength={8} onChange={(event) => setField("confirmPassword", event.target.value)} required type="password" value={draft.confirmPassword} variant="surface" />
              </UiField>
            </div>

            {validationKey ? <p className="text-xs text-(--destructive)">{t(validationKey)}</p> : null}
            {failure ? (
              <UiInlineNotice
                aria-live="assertive"
                message={t("setup.failed_description")}
                role="alert"
                title={t("setup.failed_title")}
                tone="danger"
              />
            ) : null}

            <UiButton aria-busy={isSubmitting || undefined} className="w-full" disabled={Boolean(validationKey) || isSubmitting} size="lg" tone="primary" type="submit" variant="solid">
              {isSubmitting ? t("setup.submitting") : t("setup.submit")}
              <ArrowRight className="h-4 w-4" />
            </UiButton>
          </form>
        </section>
      </div>
    </main>
  );
}

function validateSetupDraft(draft: SetupDraft): "setup.validation_capability" | "setup.validation_password" | "setup.validation_confirm" | null {
  if (draft.setupToken.length < 32) {
    return "setup.validation_capability";
  }
  if (draft.password.length < 8) {
    return "setup.validation_password";
  }
  if (draft.password !== draft.confirmPassword) {
    return "setup.validation_confirm";
  }
  return null;
}
