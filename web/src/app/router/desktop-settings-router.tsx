import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { AuthGuard } from "@/app/router/auth-guard";
import { DesktopEntryFallback, DesktopEntryLayout } from "@/app/router/desktop-entry-layout";
import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { OnboardingTourProvider } from "@/shared/ui/onboarding/tour-provider";

const LoginPage = lazy(() =>
  import("@/pages/login/login-page").then((m) => ({ default: m.LoginPage })),
);
const SettingsPage = lazy(() =>
  import("@/pages/settings/settings-page").then((m) => ({ default: m.SettingsPage })),
);

export function DesktopSettingsRouter() {
  return (
    <BrowserRouter>
      <OnboardingTourProvider>
        <Suspense fallback={<DesktopEntryFallback />}>
          <Routes>
            <Route element={<LoginPage />} path={APP_ROUTE_PATHS.login} />
            <Route element={<AuthGuard />}>
              <Route element={<DesktopEntryLayout />}>
                <Route element={<SettingsPage standalone />} path={APP_ROUTE_PATHS.settings} />
              </Route>
            </Route>
            <Route element={<Navigate replace to={APP_ROUTE_PATHS.settings} />} path="*" />
          </Routes>
        </Suspense>
      </OnboardingTourProvider>
    </BrowserRouter>
  );
}
