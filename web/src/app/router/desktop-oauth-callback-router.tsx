import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { DesktopEntryFallback } from "@/app/router/desktop-entry-layout";
import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";

const ConnectorOAuthCallbackPage = lazy(() =>
  import("@/pages/connectors/connector-oauth-callback-page").then((m) => ({
    default: m.ConnectorOAuthCallbackPage,
  })),
);

// 回调页不需要在线会话：POST /connectors/oauth/callback 已对桌面 token bypass，
// 后端 CompleteOAuthCallback 从 OAuth state 自解析 owner。系统浏览器没有 Nexus 登录态，
// 套 AuthGuard 会让 auth/status 401 把回调页挡住，token 交换永远跑不到。
export function DesktopOAuthCallbackRouter() {
  return (
    <BrowserRouter>
      <Suspense fallback={<DesktopEntryFallback />}>
        <Routes>
          <Route
            element={<ConnectorOAuthCallbackPage />}
            path={APP_ROUTE_PATHS.connectorsOauthCallback}
          />
          <Route element={<Navigate replace to={APP_ROUTE_PATHS.connectorsOauthCallback} />} path="*" />
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
