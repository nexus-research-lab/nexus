import { FormEvent, useMemo, useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  Compass,
  KeyRound,
  PanelRightOpen,
  ShieldCheck,
} from "lucide-react";
import { Link, Navigate, useNavigate, useSearchParams } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button";
import { UiInput } from "@/shared/ui/form-control";

const loginSignalItems = [
  {
    title: "Launcher",
    copy: "Route work to the right room, DM, or app surface.",
    Icon: Compass,
  },
  {
    title: "Workspace",
    copy: "Keep files, history, and review context in one place.",
    Icon: PanelRightOpen,
  },
  {
    title: "Control",
    copy: "Open Nexus with one authenticated operating surface.",
    Icon: ShieldCheck,
  },
] as const;

function resolveRedirectPath(rawRedirect: string | null): string {
  if (!rawRedirect || !rawRedirect.startsWith("/")) {
    return APP_ROUTE_PATHS.launcher;
  }
  if (rawRedirect === APP_ROUTE_PATHS.login || rawRedirect === APP_ROUTE_PATHS.landing) {
    return APP_ROUTE_PATHS.launcher;
  }
  return rawRedirect;
}

function LoginBackground() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none absolute inset-0 bg-[#ededec] bg-[linear-gradient(rgba(255,255,255,0.18),rgba(255,255,255,0.18)),linear-gradient(90deg,rgba(255,255,255,0.46)_1px,transparent_1px),linear-gradient(60deg,rgba(255,255,255,0.42)_1px,transparent_1px),linear-gradient(120deg,rgba(255,255,255,0.42)_1px,transparent_1px)] bg-[length:100%_100%,160px_138px,160px_138px,160px_138px]"
    />
  );
}

export function LoginPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const redirectPath = useMemo(
    () => resolveRedirectPath(searchParams.get("redirect")),
    [searchParams],
  );
  const { status, loading, isBootstrapped: isBootstrapped, error, login, refreshStatus: refreshStatus } = useAuth();
  const handleRefresh = () => {
    void refreshStatus().catch((err: unknown) =>
      console.warn("[LoginPage] Auth refresh failed:", err),
    );
  };
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  if (!isBootstrapped) {
    return (
      <main className="relative min-h-screen overflow-hidden bg-[#ededec] text-foreground">
        <LoginBackground />
      </main>
    );
  }

  if (!loading && status && (!status.auth_required || status.authenticated)) {
    return <Navigate replace to={redirectPath} />;
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSubmitting(true);
    setSubmitError(null);

    try {
      await login(username, password);
      navigate(redirectPath, { replace: true });
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : t("login.unknown_error"));
    } finally {
      setIsSubmitting(false);
    }
  };

  const showDisabledState = !!status && status.auth_required && !status.password_login_enabled;

  return (
    <main className="relative min-h-screen overflow-hidden bg-[#ededec] px-5 py-8 text-foreground sm:px-8 lg:px-10">
      <LoginBackground />

      <div className="relative mx-auto grid min-h-[calc(100vh-4rem)] w-full max-w-[1180px] grid-cols-1 items-center gap-8 lg:grid-cols-[minmax(0,0.96fr)_minmax(360px,430px)] lg:gap-16">
        <section className="relative min-w-0 py-6">
          <Link
            aria-label="Back to Nexus landing"
            className="inline-flex items-center gap-3 text-(--text-strong) no-underline"
            to={APP_ROUTE_PATHS.landing}
          >
            <img
              alt=""
              className="h-10 w-10 object-contain drop-shadow-[0_12px_24px_rgba(91,114,255,0.18)]"
              src="/logo.webp"
            />
            <span className="text-[28px] font-semibold leading-none">NEXUS</span>
          </Link>

          <div
            aria-hidden="true"
            className="pointer-events-none absolute -top-10 right-[72px] hidden lg:block xl:right-24"
          >
            <div className="absolute bottom-2 left-8 h-[74px] w-[144px] rounded-full bg-[rgba(91,114,255,0.10)] blur-2xl" />
            <img
              alt=""
              className="relative h-auto w-[228px] drop-shadow-[0_22px_30px_rgba(91,114,255,0.15)] xl:w-[246px]"
              src="/nexus/relaxing-generated.png"
            />
          </div>

          <div className="mt-10 max-w-[620px] sm:mt-14 lg:mt-20">
            <p className="text-sm font-semibold text-(--text-soft)">Private workspace access</p>
            <h1 className="mt-4 max-w-[560px] text-[44px] font-semibold leading-[0.98] text-[#17212c] sm:text-[64px]">
              Enter the operating surface.
            </h1>
            <p className="mt-6 max-w-[520px] text-[17px] leading-8 text-[rgba(66,81,98,0.76)]">
              Sign in to open the launcher, rooms, workspace files, and review surfaces that keep
              agent work visible.
            </p>
          </div>

          <div className="mt-10 hidden max-w-[680px] gap-3 sm:grid sm:grid-cols-3">
            {loginSignalItems.map(({ title, copy, Icon }) => (
              <div
                className="min-w-0 border-t border-[rgba(117,131,149,0.18)] bg-white/20 px-1 py-4"
                key={title}
              >
                <div className="flex items-center gap-2 text-[#17212c]">
                  <Icon className="h-4 w-4 text-[rgba(91,114,255,0.88)]" />
                  <strong className="text-sm font-semibold">{title}</strong>
                </div>
                <p className="mt-2 text-[13px] leading-5 text-[rgba(66,81,98,0.72)]">{copy}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="relative w-full overflow-hidden rounded-[12px] border border-white/70 bg-[rgba(255,255,255,0.62)] p-6 shadow-[0_26px_58px_rgba(94,108,127,0.14),0_3px_14px_rgba(94,108,127,0.07)] backdrop-blur-xl sm:p-7">
          <div className="flex items-start justify-between gap-5">
            <div className="min-w-0">
              <div className="inline-flex items-center gap-2 rounded-[10px] border border-[rgba(117,131,149,0.18)] bg-white/50 px-2.5 py-1.5 text-xs font-semibold text-[rgba(35,49,63,0.76)]">
                <KeyRound className="h-3.5 w-3.5 text-[rgba(91,114,255,0.9)]" />
                Secure session
              </div>
              <h2 className="mt-5 text-[28px] font-semibold leading-tight text-[#17212c]">
                {t("login.title")}
              </h2>
              <p className="mt-2 text-sm leading-6 text-(--text-muted)">
                Use your Nexus password to continue.
              </p>
            </div>
            <img
              alt=""
              className="h-12 w-12 shrink-0 object-contain drop-shadow-[0_12px_24px_rgba(91,114,255,0.16)]"
              src="/logo.webp"
            />
          </div>

          {error ? (
            <div className="mt-5 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-4 py-3 text-sm text-(--destructive)">
              {error}
            </div>
          ) : null}

          {showDisabledState ? (
            <div className="mt-7 space-y-4">
              <div className="rounded-[10px] border border-(--divider-subtle-color) bg-white/40 px-4 py-4">
                <h3 className="text-base font-semibold text-(--text-strong)">
                  {t("login.disabled_title")}
                </h3>
                <p className="mt-2 text-sm leading-6 text-(--text-muted)">
                  {t("login.disabled_description")}
                </p>
              </div>

              <UiButton
                className="min-h-11 w-full rounded-[10px] px-5 text-sm"
                onClick={handleRefresh}
                size="lg"
                variant="solid"
              >
                {t("login.refresh")}
              </UiButton>
            </div>
          ) : (
            <form className="mt-7 space-y-4" onSubmit={handleSubmit}>
              <label className="block" htmlFor="nexus-login-username">
                <span className="mb-2 block text-sm font-semibold text-(--text-default)">
                  {t("login.username")}
                </span>
                <UiInput
                  autoComplete="username"
                  className="min-h-12 rounded-[10px] border-[rgba(117,131,149,0.2)] bg-white/60 px-4 text-base shadow-none"
                  controlSize="lg"
                  id="nexus-login-username"
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder={t("login.username_placeholder")}
                  type="text"
                  variant="surface"
                  value={username}
                />
              </label>

              <label className="block" htmlFor="nexus-login-password">
                <span className="mb-2 block text-sm font-semibold text-(--text-default)">
                  {t("login.password")}
                </span>
                <UiInput
                  autoComplete="current-password"
                  className="min-h-12 rounded-[10px] border-[rgba(117,131,149,0.2)] bg-white/60 px-4 text-base shadow-none"
                  controlSize="lg"
                  id="nexus-login-password"
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder={t("login.password_placeholder")}
                  type="password"
                  variant="surface"
                  value={password}
                />
              </label>

              {submitError ? (
                <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-4 py-3 text-sm text-(--destructive)">
                  {submitError}
                </div>
              ) : null}

              <UiButton
                className="min-h-12 w-full rounded-[10px] px-5 text-base shadow-[0_14px_30px_rgba(23,33,44,0.14)]"
                disabled={isSubmitting}
                size="lg"
                tone="primary"
                type="submit"
                variant="solid"
              >
                <span>{isSubmitting ? t("login.submitting") : t("login.submit")}</span>
                <ArrowRight className="h-4 w-4" />
              </UiButton>
            </form>
          )}

          <div className="mt-7 flex items-center gap-2 border-t border-[rgba(117,131,149,0.16)] pt-4 text-xs leading-5 text-[rgba(66,81,98,0.72)]">
            <CheckCircle2 className="h-4 w-4 shrink-0 text-[rgba(79,162,159,0.9)]" />
            Authenticated sessions open the launcher without exposing public entry actions.
          </div>
        </section>
      </div>
    </main>
  );
}
