// INPUT: 当前 URL、认证边界与按需加载的页面模块。
// OUTPUT: Nexus Web 的唯一产品路由树，以及遵循共享 Spinner 规范的页面加载占位。
// POS: App 路由组合层；不拥有页面业务、认证数据或加载图标 recipe。

import { LoaderCircle } from "lucide-react";
import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { AuthGuard } from "@/app/router/auth-guard";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { OnboardingTourProvider } from "@/shared/ui/onboarding/tour-provider";

const AuthenticatedAppSessionRoot = lazy(() =>
  import("@/app/router/authenticated-session-root").then((m) => ({
    default: m.AuthenticatedAppSessionRoot,
  })),
);
const AppLayout = lazy(() =>
  import("@/app/layout/app-layout").then((m) => ({ default: m.AppLayout })),
);

// 懒加载页面组件 — 首次导航时按需加载
const LoginPage = lazy(() =>
  import("@/pages/login/login-page").then((m) => ({ default: m.LoginPage })),
);
const SetupPage = lazy(() =>
  import("@/pages/setup/setup-page").then((m) => ({ default: m.SetupPage })),
);
const LauncherPage = lazy(() =>
  import("@/pages/launcher/launcher-page").then((m) => ({ default: m.LauncherPage })),
);
const ContactsPage = lazy(() =>
  import("@/pages/contacts/contacts-page").then((m) => ({ default: m.ContactsPage })),
);
const HomePage = lazy(() =>
  import("@/pages/home/home-page").then((m) => ({ default: m.HomePage })),
);
const RoomPage = lazy(() =>
  import("@/pages/room/room-page").then((m) => ({ default: m.RoomPage })),
);
const ScheduledTasksPage = lazy(() =>
  import("@/pages/scheduled-tasks/scheduled-tasks-page").then((m) => ({ default: m.ScheduledTasksPage })),
);
const ChannelsPage = lazy(() =>
  import("@/pages/channels/channels-page").then((m) => ({ default: m.ChannelsPage })),
);
const PairingsPage = lazy(() =>
  import("@/pages/pairings/pairings-page").then((m) => ({ default: m.PairingsPage })),
);
const SkillsPage = lazy(() =>
  import("@/pages/skills/skills-page").then((m) => ({ default: m.SkillsPage })),
);
const ConnectorsPage = lazy(() =>
  import("@/pages/connectors/connectors-page").then((m) => ({ default: m.ConnectorsPage })),
);
const LoopsPage = lazy(() =>
  import("@/pages/loops/loops-page").then((m) => ({ default: m.LoopsPage })),
);
const WorkGraphDistillationsPage = lazy(() =>
  import("@/pages/workgraph-distillations/workgraph-distillations-page").then((m) => ({ default: m.WorkGraphDistillationsPage })),
);
const ConnectorOAuthCallbackPage = lazy(() =>
  import("@/pages/connectors/connector-oauth-callback-page").then((m) => ({
    default: m.ConnectorOAuthCallbackPage,
  })),
);
const SettingsPage = lazy(() =>
  import("@/pages/settings/settings-page").then((m) => ({ default: m.SettingsPage })),
);
const OperationsPage = lazy(() =>
  import("@/pages/operations/operations-page").then((m) => ({ default: m.OperationsPage })),
);

/** 页面加载占位 */
function PageFallback() {
  const { t } = useI18n();

  return (
    <div
      aria-busy="true"
      aria-label={t("common.loading")}
      className="flex h-full items-center justify-center"
      role="status"
    >
      <LoaderCircle
        aria-hidden
        className={getUiSpinnerClassName({ size: "xl", tone: "primary" })}
      />
    </div>
  );
}

export function AppRouter() {
  return (
    <BrowserRouter>
      <OnboardingTourProvider>
        <Suspense fallback={<PageFallback />}>
          <Routes>
            <Route
              element={<Navigate replace to={APP_ROUTE_PATHS.launcher} />}
              path={APP_ROUTE_PATHS.root}
            />
            <Route element={<LoginPage />} path={APP_ROUTE_PATHS.login} />
            <Route element={<SetupPage />} path={APP_ROUTE_PATHS.setup} />

            <Route element={<AuthGuard />}>
              <Route
                element={<ConnectorOAuthCallbackPage />}
                path={APP_ROUTE_PATHS.connectorsOauthCallback}
              />

              <Route element={<AuthenticatedAppSessionRoot />}>
                {/* Launcher — 无侧边栏布局，主窗口只有进入该路由时才加载 */}
                <Route element={<AppLayout showSidebar={false} />} path={APP_ROUTE_PATHS.launcher}>
                  <Route index element={<LauncherPage />} />
                </Route>

                {/* 有侧边栏的页面 — 共享 AppLayout，路由切换时侧边栏不重新挂载 */}
                <Route element={<AppLayout />}>
                  <Route element={<HomePage />} path={APP_ROUTE_PATHS.home} />

                  {/* Room 路由 */}
                  <Route element={<RoomPage />} path={APP_ROUTE_PATHS.room} />
                  <Route element={<RoomPage />} path={APP_ROUTE_PATHS.roomConversation} />
                  <Route element={<RoomPage />} path={APP_ROUTE_PATHS.roomSession} />

                  {/* /rooms 独立路由重定向到 /app */}
                  <Route element={<Navigate replace to={APP_ROUTE_PATHS.home} />} path="/rooms" />

                  {/* Contacts 路由 */}
                  <Route element={<ContactsPage />} path={APP_ROUTE_PATHS.contacts} />

                  {/* Skills 路由 */}
                  <Route
                    element={<Navigate replace to={APP_ROUTE_PATHS.skills} />}
                    path={APP_ROUTE_PATHS.capability}
                  />
                  <Route element={<SkillsPage />} path={APP_ROUTE_PATHS.skills} />
                  <Route element={<SkillsPage />} path={APP_ROUTE_PATHS.skillDetail} />

                  {/* 能力子路由 */}
                  <Route element={<LoopsPage />} path={APP_ROUTE_PATHS.loops} />
                  <Route element={<LoopsPage />} path={APP_ROUTE_PATHS.loopDetail} />
                  <Route element={<WorkGraphDistillationsPage />} path={APP_ROUTE_PATHS.workGraphDistillations} />
                  <Route element={<WorkGraphDistillationsPage />} path={APP_ROUTE_PATHS.workGraphDistillationDetail} />
                  <Route element={<ConnectorsPage />} path={APP_ROUTE_PATHS.connectors} />
                  <Route element={<ConnectorsPage />} path={APP_ROUTE_PATHS.connectorDetail} />
                  <Route element={<ScheduledTasksPage />} path={APP_ROUTE_PATHS.scheduledTasks} />
                  <Route element={<ChannelsPage />} path={APP_ROUTE_PATHS.channels} />
                  <Route element={<PairingsPage />} path={APP_ROUTE_PATHS.pairings} />

                  {/* 其他占位路由 */}
                  <Route element={<OperationsPage />} path={APP_ROUTE_PATHS.operations} />
                  <Route element={<OperationsPage />} path={APP_ROUTE_PATHS.adminMembers} />
                  <Route element={<SettingsPage />} path={APP_ROUTE_PATHS.settings} />
                </Route>
              </Route>
            </Route>

            {/* 兜底重定向 */}
            <Route element={<Navigate replace to={APP_ROUTE_PATHS.launcher} />} path="*" />
          </Routes>
        </Suspense>
      </OnboardingTourProvider>
    </BrowserRouter>
  );
}
