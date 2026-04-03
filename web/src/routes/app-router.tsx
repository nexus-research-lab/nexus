import { lazy, Suspense } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { LauncherPage } from "@/pages/launcher/launcher-page";
import { PlaceholderPage } from "@/pages/placeholder/placeholder-page";
import { AppLayout } from "@/shared/ui/layout/app-layout";
import { AppLayoutNoSidebar } from "@/shared/ui/layout/app-layout-no-sidebar";

// 懒加载页面组件 — 首次导航时按需加载
const ContactsPage = lazy(() =>
  import("@/pages/contacts/contacts-page").then((m) => ({ default: m.ContactsPage })),
);
const DmsPage = lazy(() =>
  import("@/pages/dms/dms-page").then((m) => ({ default: m.DmsPage })),
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
const SkillsPage = lazy(() =>
  import("@/pages/skills/skills-page").then((m) => ({ default: m.SkillsPage })),
);
const ConnectorsPage = lazy(() =>
  import("@/pages/connectors/connectors-page").then((m) => ({ default: m.ConnectorsPage })),
);

/** 页面加载占位 */
function PageFallback() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
    </div>
  );
}

export function AppRouter() {
  return (
    <BrowserRouter>
      <Suspense fallback={<PageFallback />}>
        <Routes>
          {/* Launcher — 无侧边栏布局，eager 加载 */}
          <Route element={<AppLayoutNoSidebar />} path={APP_ROUTE_PATHS.launcher}>
            <Route index element={<LauncherPage />} />
          </Route>

          {/* 有侧边栏的页面 — 共享 AppLayout，路由切换时侧边栏不重新挂载 */}
          <Route element={<AppLayout />}>
            <Route element={<HomePage />} path={APP_ROUTE_PATHS.home} />
            <Route element={<DmsPage />} path={APP_ROUTE_PATHS.dm_directory} />

            {/* Room 路由 */}
            <Route element={<RoomPage />} path={APP_ROUTE_PATHS.room} />
            <Route element={<RoomPage />} path={APP_ROUTE_PATHS.room_conversation} />

            {/* /rooms 独立路由重定向到 /app */}
            <Route element={<Navigate replace to={APP_ROUTE_PATHS.home} />} path="/rooms" />

            {/* Contacts 路由 */}
            <Route element={<ContactsPage />} path={APP_ROUTE_PATHS.contacts} />
            <Route element={<ContactsPage />} path={APP_ROUTE_PATHS.contact_profile} />

            {/* Skills 路由 */}
            <Route element={<SkillsPage />} path={APP_ROUTE_PATHS.skills} />
            <Route element={<SkillsPage />} path={APP_ROUTE_PATHS.skill_detail} />

            {/* 能力子路由 */}
            <Route element={<ConnectorsPage />} path={APP_ROUTE_PATHS.connectors} />
            <Route element={<ScheduledTasksPage />} path={APP_ROUTE_PATHS.scheduled_tasks} />
            <Route element={<PlaceholderPage title="频道" description="管理消息频道" />} path={APP_ROUTE_PATHS.channels} />
            <Route element={<PlaceholderPage title="配对" description="管理 Agent 配对关系" />} path={APP_ROUTE_PATHS.pairings} />

            {/* 其他占位路由 */}
            <Route element={<PlaceholderPage title="设置" description="应用程序设置和偏好" />} path={APP_ROUTE_PATHS.settings} />
          </Route>

          {/* 兜底重定向 */}
          <Route element={<Navigate replace to={APP_ROUTE_PATHS.launcher} />} path="*" />
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
