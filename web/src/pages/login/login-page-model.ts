import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { projectMutationFailure } from "@/lib/error-message";
import type { AuthStatus } from "@/lib/api/account/auth-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

const INTERNAL_REDIRECT_ORIGIN = "https://nexus.local";
const REDIRECT_FALLBACK_PATHS = new Set<string>([
  APP_ROUTE_PATHS.landing,
  APP_ROUTE_PATHS.login,
]);

export type LoginFormMode = "disabled" | "password";

export interface LoginRecoveryNotice {
  action: "check_status" | null;
  blocksSubmit: boolean;
  impact: string;
  nextStep: string;
  title: string;
}

export type LoginPageState =
  | { kind: "bootstrapping" }
  | { kind: "redirect"; path: string }
  | { kind: "ready"; formMode: LoginFormMode };

interface LoginPageStateOptions {
  isBootstrapped: boolean;
  loading: boolean;
  redirectPath: string;
  status: AuthStatus | null;
}

export function resolveLoginRedirectPath(rawRedirect: string | null): string {
  if (!rawRedirect?.startsWith("/")) {
    return APP_ROUTE_PATHS.launcher;
  }
  try {
    const redirect = new URL(rawRedirect, INTERNAL_REDIRECT_ORIGIN);
    if (
      redirect.origin !== INTERNAL_REDIRECT_ORIGIN
      || REDIRECT_FALLBACK_PATHS.has(redirect.pathname)
    ) {
      return APP_ROUTE_PATHS.launcher;
    }
    return `${redirect.pathname}${redirect.search}${redirect.hash}`;
  } catch {
    return APP_ROUTE_PATHS.launcher;
  }
}

export function buildLoginPageState({
  isBootstrapped,
  loading,
  redirectPath,
  status,
}: LoginPageStateOptions): LoginPageState {
  if (!isBootstrapped) {
    return { kind: "bootstrapping" };
  }
  if (shouldRedirectAuthenticatedSession(status, loading)) {
    return { kind: "redirect", path: redirectPath };
  }
  return {
    formMode: isPasswordLoginDisabled(status) ? "disabled" : "password",
    kind: "ready",
  };
}

export function buildLoginSubmitFailure(
  error: unknown,
  t: I18nContextValue["t"],
): LoginRecoveryNotice {
  const failure = projectMutationFailure(error, t("login.unknown_error"));
  if (failure.effect === "not_applied") {
    return {
      action: null,
      blocksSubmit: false,
      impact: t("login.submit_not_applied_impact"),
      nextStep: t("login.submit_not_applied_next_step"),
      title: t("login.submit_failed_title"),
    };
  }
  return {
    action: "check_status",
    blocksSubmit: true,
    impact: t("login.submit_unknown_impact"),
    nextStep: t("login.submit_unknown_next_step"),
    title: t("login.submit_unknown_title"),
  };
}

export function buildLoginStatusFailure(
  _message: string,
  hasKnownStatus: boolean,
  t: I18nContextValue["t"],
): LoginRecoveryNotice {
  return {
    action: "check_status",
    blocksSubmit: !hasKnownStatus,
    impact: t(hasKnownStatus
      ? "login.runtime_options_failure_impact"
      : "state.read_failure_impact"),
    nextStep: t(hasKnownStatus
      ? "login.runtime_options_failure_next_step"
      : "state.retry_next_step"),
    title: t(hasKnownStatus
      ? "login.runtime_options_failure_title"
      : "login.status_failure_title"),
  };
}

function shouldRedirectAuthenticatedSession(
  status: AuthStatus | null,
  loading: boolean,
): boolean {
  return !loading
    && status !== null
    && (!status.auth_required || status.authenticated);
}

function isPasswordLoginDisabled(status: AuthStatus | null): boolean {
  return status?.auth_required === true && !status.password_login_enabled;
}
