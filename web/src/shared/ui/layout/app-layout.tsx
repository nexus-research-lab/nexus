/**
 * 应用布局路由组件
 *
 * 使用 React Router <Outlet /> 渲染子路由内容。
 * 侧边栏直接挂在路由布局层，避免路由切换时被卸载/重新挂载。
 *
 * show_sidebar=false 用于 LauncherPage 等不需要侧边栏的页面。
 */

import { Outlet } from "react-router-dom";

import { HOME_PAGE_PADDING_CLASS } from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { SidebarWidePanel } from "@/shared/ui/sidebar/sidebar-wide-panel";

export function AppLayout({ show_sidebar = true }: { show_sidebar?: boolean }) {
  return (
    <main className="relative flex h-screen w-full overflow-hidden bg-transparent text-foreground">
      {show_sidebar ? <SidebarWidePanel /> : null}
      <div className={cn("relative flex min-h-0 flex-1 flex-col overflow-hidden", HOME_PAGE_PADDING_CLASS)}>
        <Outlet />
      </div>
    </main>
  );
}
