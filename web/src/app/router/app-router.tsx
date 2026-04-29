import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { get_agent_ws_url } from "@/config/options";
import { useWebSocket } from "@/lib/websocket";
import { LauncherPage } from "@/pages/launcher/launcher-page";
import { LoginPage } from "@/pages/login/login-page";
import { AuthGuard } from "@/app/router/auth-guard";
import { AppLayout } from "@/shared/ui/layout/app-layout";
import { OnboardingTourProvider } from "@/shared/ui/onboarding/tour-provider";

// 懒加载页面组件 — 首次导航时按需加载
const ContactsPage = lazy(() =>
  import("@/pages/contacts/contacts-page").then((m) => ({default: m.ContactsPage})),
);
const HomePage = lazy(() =>
  import("@/pages/home/home-page").then((m) => ({default: m.HomePage})),
);
const RoomPage = lazy(() =>
  import("@/pages/room/room-page").then((m) => ({default: m.RoomPage})),
);
const ScheduledTasksPage = lazy(() =>
  import("@/pages/scheduled-tasks/scheduled-tasks-page").then((m) => ({default: m.ScheduledTasksPage})),
);
const ChannelsPage = lazy(() =>
  import("@/pages/channels/channels-page").then((m) => ({default: m.ChannelsPage})),
);
const PairingsPage = lazy(() =>
  import("@/pages/pairings/pairings-page").then((m) => ({default: m.PairingsPage})),
);
const SkillsPage = lazy(() =>
  import("@/pages/skills/skills-page").then((m) => ({default: m.SkillsPage})),
);
const ConnectorsPage = lazy(() =>
  import("@/pages/connectors/connectors-page").then((m) => ({default: m.ConnectorsPage})),
);
const ConnectorOAuthCallbackPage = lazy(() =>
  import("@/pages/connectors/connector-oauth-callback-page").then((m) => ({
    default: m.ConnectorOAuthCallbackPage,
  })),
);
const SettingsPage = lazy(() =>
  import("@/pages/settings/settings-page").then((m) => ({default: m.SettingsPage})),
);

/** 页面加载占位 */
function PageFallback() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"/>
    </div>
  );
}

function AuthenticatedAppSessionRoot() {
  const ws_url = get_agent_ws_url();

  useWebSocket({
    url: ws_url,
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
  });

  return <Outlet/>;
}

export function AppRouter() {
  return (
    <BrowserRouter>
      <OnboardingTourProvider>
        <Suspense fallback={<PageFallback/>}>
          <Routes>
            <Route element={<LoginPage/>} path={APP_ROUTE_PATHS.login}/>

            <Route element={<AuthGuard/>}>
              <Route
                element={<ConnectorOAuthCallbackPage/>}
                path={APP_ROUTE_PATHS.connectors_oauth_callback}
              />

              <Route element={<AuthenticatedAppSessionRoot/>}>
                {/* Launcher — 无侧边栏布局，eager 加载 */}
                <Route element={<AppLayout show_sidebar={false}/>} path={APP_ROUTE_PATHS.launcher}>
                  <Route index element={<LauncherPage/>}/>
                </Route>

                {/* 有侧边栏的页面 — 共享 AppLayout，路由切换时侧边栏不重新挂载 */}
                <Route element={<AppLayout/>}>
                  <Route element={<HomePage/>} path={APP_ROUTE_PATHS.home}/>

                  {/* Room 路由 */}
                  <Route element={<RoomPage/>} path={APP_ROUTE_PATHS.room}/>
                  <Route element={<RoomPage/>} path={APP_ROUTE_PATHS.room_conversation}/>

                  {/* /rooms 独立路由重定向到 /app */}
                  <Route element={<Navigate replace to={APP_ROUTE_PATHS.home}/>} path="/rooms"/>

                  {/* Contacts 路由 */}
                  <Route element={<ContactsPage/>} path={APP_ROUTE_PATHS.contacts}/>

                  {/* Skills 路由 */}
                  <Route element={<SkillsPage/>} path={APP_ROUTE_PATHS.skills}/>
                  <Route element={<SkillsPage/>} path={APP_ROUTE_PATHS.skill_detail}/>

                  {/* 能力子路由 */}
                  <Route element={<ConnectorsPage/>} path={APP_ROUTE_PATHS.connectors}/>
                  <Route element={<ScheduledTasksPage/>} path={APP_ROUTE_PATHS.scheduled_tasks}/>
                  <Route element={<ChannelsPage/>} path={APP_ROUTE_PATHS.channels}/>
                  <Route element={<PairingsPage/>} path={APP_ROUTE_PATHS.pairings}/>

                  {/* 其他占位路由 */}
                  <Route element={<SettingsPage/>} path={APP_ROUTE_PATHS.settings}/>
                </Route>
              </Route>
            </Route>

            {/* 兜底重定向 */}
            <Route element={<Navigate replace to={APP_ROUTE_PATHS.launcher}/>} path="*"/>
          </Routes>
        </Suspense>
      </OnboardingTourProvider>
    </BrowserRouter>
  );
}
