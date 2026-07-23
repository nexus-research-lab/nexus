/**
 * 应用布局路由组件
 *
 * 使用 React Router <Outlet /> 渲染子路由内容。
 * 侧边栏直接挂在路由布局层，避免路由切换时被卸载/重新挂载。
 *
 * showSidebar=false 用于 LauncherPage 等不需要侧边栏的页面。
 */

import { Outlet, useLocation, useNavigate } from "react-router-dom";

import { SidebarWidePanel } from "@/features/navigation/sidebar/sidebar-wide-panel";
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import {
  CONVERSATION_FOCUS_MEDIA_QUERY,
  HOME_PAGE_PADDING_CLASS,
} from "@/lib/layout/home-layout";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import { MobileAppPageHeader } from "./mobile-app-page-header";
import { resolveMobileAppRoute } from "./mobile-app-route-model";

export function AppLayout({ showSidebar = true }: { showSidebar?: boolean }) {
  const { pathname, search } = useLocation();
  const navigate = useNavigate();
  const { t } = useI18n();
  const isNarrowViewport = useMediaQuery(CONVERSATION_FOCUS_MEDIA_QUERY);
  const mobileRoute = resolveMobileAppRoute({ pathname, search });
  const isMobileAppLayout = showSidebar && isNarrowViewport;
  const isMobileDirectory = (
    isMobileAppLayout
    && mobileRoute.mode === "directory"
  );
  const isMobileContent = (
    isMobileAppLayout
    && mobileRoute.mode !== "directory"
  );

  return (
    <main className="desktop-window-frame relative flex h-dvh w-full overflow-hidden bg-transparent text-foreground">
      {showSidebar && (!isMobileAppLayout || isMobileDirectory) ? (
        <SidebarWidePanel
          fillAvailableWidth={isMobileDirectory}
          navigationOnly={isMobileDirectory}
        />
      ) : null}
      {!isMobileDirectory ? (
        <div className={cn(
          "relative flex min-h-0 flex-1 flex-col overflow-hidden",
          !isMobileContent && HOME_PAGE_PADDING_CLASS,
        )}>
          {isMobileAppLayout && mobileRoute.mode === "detail" ? (
            <MobileAppPageHeader
              onBack={() => navigate(mobileRoute.backPath)}
              title={t(mobileRoute.titleKey)}
            />
          ) : null}
          <Outlet />
        </div>
      ) : null}
    </main>
  );
}
