// INPUT: Control-backed 认证状态、当前路由和重新读取认证状态命令。
// OUTPUT: 加载/恢复状态或通往 setup、login、受保护路由的唯一入口。
// POS: App 路由认证守卫；不拥有认证数据、通用反馈或按钮视觉。

import { Navigate, Outlet, useLocation } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { useAuth } from "@/shared/auth/auth-context";
import { UiButton } from "@/shared/ui/button/button";
import { AppLoadingState } from "@/shared/ui/layout/app-loading-screen";

function GuardState({
  title,
  description,
  impact,
  nextStep,
  actionLabel: actionLabel,
  onAction: onAction,
}: {
  title: string;
  description: string;
  impact?: string;
  nextStep?: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
      <section className="surface-panel surface-radius-xl w-full max-w-[440px] border px-8 py-9 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background) text-lg font-semibold">
          N
        </div>
        <h1 className="text-lg font-semibold text-(--text-strong)">{title}</h1>
        <p className="mt-2 text-base leading-6 text-(--text-muted)">{description}</p>
        {impact ? (
          <p className="mt-3 text-sm leading-6 text-(--text-muted)">{impact}</p>
        ) : null}
        {nextStep ? (
          <p className="mt-2 text-sm font-medium leading-6 text-(--text-default)">{nextStep}</p>
        ) : null}
        {actionLabel && onAction ? (
          <UiButton
            className="mt-5"
            onClick={onAction}
            size="lg"
            tone="primary"
            variant="solid"
          >
            {actionLabel}
          </UiButton>
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
        impact="读取登录状态不会修改账号或工作区中已保存的数据。"
        nextStep="检查网络连接后重新加载登录状态；如果仍然失败，请稍后再试。"
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
        impact="当前页面不会继续加载账号内容，已保存的数据不会被修改。"
        nextStep="重新加载登录状态；如果仍然没有结果，请稍后再试。"
        actionLabel="重试"
        onAction={handleRefresh}
      />
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
