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
import { getUiButtonClassName } from "@/shared/ui/button-styles";
import { AppLoadingState } from "@/shared/ui/layout/app-loading-screen";

function GuardState({
  title,
  description,
  actionLabel: actionLabel,
  onAction: onAction,
}: {
  title: string;
  description: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className="surface-panel surface-radius-xl w-full max-w-[440px] border px-8 py-9 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-bold">
          N
        </div>
        <h1 className="text-[24px] font-bold text-(--text-strong)">{title}</h1>
        <p className="mt-2 text-[14px] leading-6 text-(--text-muted)">{description}</p>
        {actionLabel && onAction ? (
          <button
            className={getUiButtonClassName(
              { size: "lg", tone: "primary", variant: "solid" },
              "mt-5 rounded-full px-5 text-[14px]",
            )}
            onClick={onAction}
            type="button"
          >
            {actionLabel}
          </button>
        ) : null}
      </section>
    </main>
  );
}

export function AuthGuard() {
  const location = useLocation();
  const { status, isBootstrapped: isBootstrapped, error, refreshStatus: refreshStatus } = useAuth();
  const handleRefresh = () => {
    void refreshStatus().catch((err: unknown) => console.warn("[AuthGuard] Auth refresh failed:", err));
  };

  if (!isBootstrapped) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
        <AppLoadingState message="正在连接 Nexus" />
      </main>
    );
  }

  if (error && !status) {
    return (
      <GuardState
        title="无法连接认证服务"
        description={error}
        actionLabel="重试"
        onAction={handleRefresh}
      />
    );
  }

  if (!status) {
    return (
      <GuardState
        title="认证状态不可用"
        description="服务端没有返回可用的登录状态，请稍后重试。"
        actionLabel="重试"
        onAction={handleRefresh}
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
