import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import type { AuthStatus } from "@/lib/api/account/auth-api";

const INTERNAL_REDIRECT_ORIGIN = "https://nexus.local";
const REDIRECT_FALLBACK_PATHS = new Set<string>([
  APP_ROUTE_PATHS.landing,
  APP_ROUTE_PATHS.login,
]);

export type LoginFormMode = "disabled" | "password";

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

export function getLoginSubmitError(
  error: unknown,
  fallback: string,
): string {
  return error instanceof Error ? error.message : fallback;
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
