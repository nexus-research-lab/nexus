/**
 * 应用布局路由组件
 *
 * 使用 React Router <Outlet /> 渲染子路由内容。
 * 侧边栏直接挂在路由布局层，避免路由切换时被卸载/重新挂载。
 *
 * showSidebar=false 用于 LauncherPage 等不需要侧边栏的页面。
 */

import { Outlet } from "react-router-dom";

import { HOME_PAGE_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn } from "@/lib/utils";
import { SidebarWidePanel } from "@/shared/ui/sidebar/sidebar-wide-panel";

export function AppLayout({ showSidebar: showSidebar = true }: { showSidebar?: boolean }) {
  return (
    <main className="relative flex h-screen w-full overflow-hidden bg-transparent text-foreground">
      {showSidebar ? <SidebarWidePanel /> : null}
      <div className={cn("relative flex min-h-0 flex-1 flex-col overflow-hidden", HOME_PAGE_PADDING_CLASS)}>
        <Outlet />
      </div>
    </main>
  );
}
