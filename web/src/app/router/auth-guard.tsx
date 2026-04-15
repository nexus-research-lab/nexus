/**
 * =====================================================
 * @File   : auth-guard.tsx
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
 */

import { Navigate, Outlet, useLocation } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { useAuth } from "@/shared/auth/auth-context";

function GuardState({
  title,
  description,
  action_label,
  on_action,
}: {
  title: string;
  description: string;
  action_label?: string;
  on_action?: () => void;
}) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className="surface-panel radius-shell-xl w-full max-w-[440px] border px-8 py-9 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-bold">
          N
        </div>
        <h1 className="text-[24px] font-bold tracking-[-0.04em] text-(--text-strong)">{title}</h1>
        <p className="mt-2 text-[14px] leading-6 text-(--text-muted)">{description}</p>
        {action_label && on_action ? (
          <button
            className="mt-5 inline-flex min-h-11 items-center justify-center rounded-full border border-(--button-primary-border) bg-(--button-primary-background) px-5 text-[14px] font-semibold text-(--button-primary-color) transition hover:bg-(--button-primary-hover-background)"
            onClick={on_action}
            type="button"
          >
            {action_label}
          </button>
        ) : null}
      </section>
    </main>
  );
}

export function AuthGuard() {
  const location = useLocation();
  const { status, is_bootstrapped, error, refresh_status } = useAuth();
  const handle_refresh = () => {
    void refresh_status().catch(() => undefined);
  };

  if (!is_bootstrapped) {
    return <main className="min-h-screen bg-background text-foreground" />;
  }

  if (error && !status) {
    return (
      <GuardState
        title="无法连接认证服务"
        description={error}
        action_label="重试"
        on_action={handle_refresh}
      />
    );
  }

  if (!status) {
    return (
      <GuardState
        title="认证状态不可用"
        description="服务端没有返回可用的登录状态，请稍后重试。"
        action_label="重试"
        on_action={handle_refresh}
      />
    );
  }

  if (!status.auth_required || status.authenticated) {
    return <Outlet />;
  }

  const redirect = `${location.pathname}${location.search}${location.hash}`;
  return (
    <Navigate
      replace
      to={`${APP_ROUTE_PATHS.login}?redirect=${encodeURIComponent(redirect)}`}
    />
  );
}
