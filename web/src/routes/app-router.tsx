import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { ActivityPage } from "@/pages/activity/activity-page";
import { ContactsPage } from "@/pages/contacts/contacts-page";
import { DmsPage } from "@/pages/dms/dms-page";
import { HomePage } from "@/pages/home/home-page";
import { LauncherPage } from "@/pages/launcher/launcher-page";
import { PlaceholderPage } from "@/pages/placeholder/placeholder-page";
import { RoomPage } from "@/pages/room/room-page";
import { ScheduledTasksPage } from "@/pages/scheduled-tasks/scheduled-tasks-page";
import { SkillsPage } from "@/pages/skills/skills-page";

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        {/* 核心路由 */}
        <Route element={<LauncherPage />} path={APP_ROUTE_PATHS.launcher} />
        <Route element={<HomePage />} path={APP_ROUTE_PATHS.home} />
        <Route element={<ActivityPage />} path={APP_ROUTE_PATHS.activity} />
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
        <Route element={<PlaceholderPage title="连接器" description="管理外部服务连接" />} path={APP_ROUTE_PATHS.connectors} />
        <Route element={<ScheduledTasksPage />} path={APP_ROUTE_PATHS.scheduled_tasks} />
        <Route element={<PlaceholderPage title="频道" description="管理消息频道" />} path={APP_ROUTE_PATHS.channels} />
        <Route element={<PlaceholderPage title="配对" description="管理 Agent 配对关系" />} path={APP_ROUTE_PATHS.pairings} />

        {/* 其他占位路由 */}
        <Route element={<PlaceholderPage title="文件" description="浏览和管理工作区文件" />} path={APP_ROUTE_PATHS.files} />
        <Route element={<PlaceholderPage title="设置" description="应用程序设置和偏好" />} path={APP_ROUTE_PATHS.settings} />

        {/* 兜底重定向 */}
        <Route element={<Navigate replace to={APP_ROUTE_PATHS.launcher} />} path="*" />
      </Routes>
    </BrowserRouter>
  );
}
