/**
 * INPUT: 当前路由、视口尺寸、侧栏可见性、移动页头动作与全局聊天完成事件。
 * OUTPUT: 稳定承载侧栏、移动端页头及动作槽、子路由和聊天完成订阅的应用布局。
 * POS: 路由布局根；通知订阅固定在此，窄屏详情页不渲染侧栏时仍接收 Room 未读。
 */

import { useState } from "react";
import { Outlet, useLocation, useNavigate } from "react-router-dom";

import { useChatCompletionNotifications } from "@/features/home/notifications/use-chat-completion-notifications";
import { SidebarWidePanel } from "@/features/navigation/sidebar/sidebar-wide-panel";
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import {
  APP_NARROW_VIEWPORT_MEDIA_QUERY,
  HOME_PAGE_PADDING_CLASS,
} from "@/lib/layout/home-layout";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import { MobileAppPageHeader } from "./mobile-app-page-header";
import { MobileAppPageHeaderActionsProvider } from "./mobile-app-page-header-actions";
import { resolveMobileAppRoute } from "./mobile-app-route-model";

export function AppLayout({ showSidebar = true }: { showSidebar?: boolean }) {
  const { pathname, search } = useLocation();
  const navigate = useNavigate();
  const { t } = useI18n();
  const isNarrowViewport = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
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
  const [mobileHeaderActionsTarget, setMobileHeaderActionsTarget] =
    useState<HTMLElement | null>(null);
  useChatCompletionNotifications();

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
          "desktop-app-stage relative flex min-h-0 flex-1 flex-col overflow-hidden",
          !isMobileContent && HOME_PAGE_PADDING_CLASS,
        )}>
          <MobileAppPageHeaderActionsProvider target={mobileHeaderActionsTarget}>
            {isMobileAppLayout && mobileRoute.mode === "detail" ? (
              <MobileAppPageHeader
                actionsRef={setMobileHeaderActionsTarget}
                onBack={() => navigate(mobileRoute.backPath)}
                title={t(mobileRoute.titleKey)}
              />
            ) : null}
            <Outlet />
          </MobileAppPageHeaderActionsProvider>
        </div>
      ) : null}
    </main>
  );
}
