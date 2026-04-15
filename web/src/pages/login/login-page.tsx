/**
 * =====================================================
 * @File   : login-page.tsx
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
 */

import { FormEvent, useMemo, useState } from "react";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";

function resolve_redirect_path(raw_redirect: string | null): string {
  if (!raw_redirect || !raw_redirect.startsWith("/")) {
    return APP_ROUTE_PATHS.launcher;
  }
  if (raw_redirect === APP_ROUTE_PATHS.login) {
    return APP_ROUTE_PATHS.launcher;
  }
  return raw_redirect;
}

export function LoginPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [search_params] = useSearchParams();
  const redirect_path = useMemo(
    () => resolve_redirect_path(search_params.get("redirect")),
    [search_params],
  );
  const { status, loading, is_bootstrapped, error, login, refresh_status } = useAuth();
  const handle_refresh = () => {
    void refresh_status().catch(() => undefined);
  };
  const [username, set_username] = useState("");
  const [password, set_password] = useState("");
  const [submit_error, set_submit_error] = useState<string | null>(null);
  const [is_submitting, set_is_submitting] = useState(false);

  if (!is_bootstrapped) {
    return <main className="min-h-screen bg-background text-foreground" />;
  }

  if (!loading && status && (!status.auth_required || status.authenticated)) {
    return <Navigate replace to={redirect_path} />;
  }

  const handle_submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    set_is_submitting(true);
    set_submit_error(null);

    try {
      await login(username, password);
      navigate(redirect_path, { replace: true });
    } catch (err) {
      set_submit_error(err instanceof Error ? err.message : t("login.unknown_error"));
    } finally {
      set_is_submitting(false);
    }
  };

  const show_disabled_state = !!status && status.auth_required && !status.password_login_enabled;

  return (
    <main className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-6 py-10 text-foreground">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(var(--primary-rgb),0.16),transparent_32%),radial-gradient(circle_at_bottom_right,rgba(196,154,108,0.16),transparent_28%)]" />
      <div className="pointer-events-none absolute left-[-12%] top-[8%] h-72 w-72 rounded-full bg-[rgba(var(--primary-rgb),0.08)] blur-3xl" />
      <div className="pointer-events-none absolute bottom-[-10%] right-[-4%] h-80 w-80 rounded-full bg-[rgba(196,154,108,0.10)] blur-3xl" />

      <section className="surface-panel radius-shell-xl relative w-full max-w-[430px] overflow-hidden border px-8 py-8 shadow-(--surface-popover-shadow)">
        {/* <div className="absolute inset-x-0 top-0 h-1 bg-[linear-gradient(90deg,rgba(var(--primary-rgb),0.88),rgba(196,154,108,0.88))]" /> */}

        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="mt-3 text-2xl font-extrabold tracking-[-0.05em] text-(--text-strong)">
              {t("login.title")}
            </h1>
          </div>
          <div className="flex h-14 w-14 items-center justify-center rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-black tracking-[-0.08em] text-(--text-strong)">
            N
          </div>
        </div>

        {error ? (
          <div className="mt-4 rounded-[18px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-4 py-3 text-sm text-(--destructive)">
            {error}
          </div>
        ) : null}

        {show_disabled_state ? (
          <div className="mt-6 space-y-4">
            <div className="rounded-[22px] border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) px-5 py-5">
              <h2 className="text-md font-semibold text-(--text-strong)">{t("login.disabled_title")}</h2>
              <p className="mt-2 text-base leading-6 text-(--text-muted)">
                {t("login.disabled_description")}
              </p>
            </div>

            <button
              className="inline-flex min-h-11 w-full items-center justify-center rounded-full border border-(--button-tonal-border) bg-(--button-tonal-background) px-5 text-base font-semibold text-(--button-tonal-color) transition hover:bg-(--button-tonal-hover-background) hover:text-(--button-tonal-hover-color)"
              onClick={handle_refresh}
              type="button"
            >
              {t("login.refresh")}
            </button>
          </div>
        ) : (
          <form className="mt-6 space-y-4" onSubmit={handle_submit}>
            <label className="block">
              <span className="mb-2 block text-sm font-semibold uppercase tracking-[0.18em] text-(--text-soft)">
                {t("login.username")}
              </span>
              <input
                autoComplete="username"
                className="input-shell min-h-12 w-full rounded-[18px] px-4 text-base text-(--text-strong) outline-none"
                onChange={(event) => set_username(event.target.value)}
                placeholder={t("login.username_placeholder")}
                type="text"
                value={username}
              />
            </label>

            <label className="block">
              <span className="mb-2 block text-sm font-semibold uppercase tracking-[0.18em] text-(--text-soft)">
                {t("login.password")}
              </span>
              <input
                autoComplete="current-password"
                className="input-shell min-h-12 w-full rounded-[18px] px-4 text-base text-(--text-strong) outline-none"
                onChange={(event) => set_password(event.target.value)}
                placeholder={t("login.password_placeholder")}
                type="password"
                value={password}
              />
            </label>

            {submit_error ? (
              <div className="rounded-[18px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-4 py-3 text-sm text-(--destructive)">
                {submit_error}
              </div>
            ) : null}

            <button
              className="inline-flex min-h-12 w-full items-center justify-center rounded-full border border-(--button-primary-border) bg-(--button-primary-background) px-5 text-base font-semibold text-(--button-primary-color) transition hover:bg-(--button-primary-hover-background) disabled:cursor-not-allowed disabled:opacity-60"
              disabled={is_submitting}
              type="submit"
            >
              {is_submitting ? t("login.submitting") : t("login.submit")}
            </button>
          </form>
        )}
      </section>
    </main>
  );
}
