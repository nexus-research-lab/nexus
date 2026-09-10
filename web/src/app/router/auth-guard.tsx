// INPUT: Control-backed 认证状态、当前路由和重新读取认证状态命令。
// OUTPUT: 加载/恢复状态或通往 setup、login、受保护路由的唯一入口。
// POS: App 路由认证守卫；不拥有认证数据、通用反馈或按钮视觉。

import { Navigate, Outlet, useLocation } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { useAuth } from "@/shared/auth/auth-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import { AppLoadingState } from "@/shared/ui/layout/app-loading-screen";

export function AuthGuard() {
  const location = useLocation();
  const { t } = useI18n();
  const { status, isBootstrapped, loading, refreshStatus } = useAuth();
  const handleRefresh = () => {
    void refreshStatus().catch((err: unknown) => console.warn("[AuthGuard] Auth refresh failed:", err));
  };

  if (!isBootstrapped) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
        <AppLoadingState message={t("auth_guard.connecting")} />
      </main>
    );
  }

  if (!status) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
        <UiResourceState
          className="w-full max-w-[440px]"
          state="error"
          title={t("login.status_failure_title")}
          impact={t("auth_guard.unavailable_impact")}
          nextStep={t("auth_guard.retry_hint")}
          primaryAction={{ label: t("state.retry"), onClick: handleRefresh, busy: loading, disabled: loading }}
        />
      </main>
    );
  }

  if (status.setup_required) {
    return <Navigate replace to={APP_ROUTE_PATHS.setup} />;
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
