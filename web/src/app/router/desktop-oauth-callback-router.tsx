import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { AuthGuard } from "@/app/router/auth-guard";
import { DesktopEntryFallback } from "@/app/router/desktop-entry-layout";
import { APP_ROUTE_PATHS } from "@/app/router/route-paths";

const ConnectorOAuthCallbackPage = lazy(() =>
  import("@/pages/connectors/connector-oauth-callback-page").then((m) => ({
    default: m.ConnectorOAuthCallbackPage,
  })),
);
const LoginPage = lazy(() =>
  import("@/pages/login/login-page").then((m) => ({ default: m.LoginPage })),
);

export function DesktopOAuthCallbackRouter() {
  return (
    <BrowserRouter>
      <Suspense fallback={<DesktopEntryFallback />}>
        <Routes>
          <Route element={<LoginPage />} path={APP_ROUTE_PATHS.login} />
          <Route element={<AuthGuard />}>
            <Route
              element={<ConnectorOAuthCallbackPage />}
              path={APP_ROUTE_PATHS.connectors_oauth_callback}
            />
          </Route>
          <Route element={<Navigate replace to={APP_ROUTE_PATHS.connectors_oauth_callback} />} path="*" />
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
