// INPUT: 桌面入口内容或嵌套路由出口。
// OUTPUT: 统一桌面窗口客户区，以及使用共享 Spinner 的模块加载占位。
// POS: Desktop App 路由壳；不拥有宿主标题栏、页面业务或加载图标 recipe。

import { LoaderCircle } from "lucide-react";
import type { ReactNode } from "react";
import { Outlet } from "react-router-dom";

import { HOME_PAGE_PADDING_CLASS } from "@/lib/layout/home-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

export function DesktopEntryLayout({
  children,
}: {
  children?: ReactNode;
}) {
  return (
    <main className="desktop-window-frame relative flex h-screen w-full overflow-hidden bg-transparent text-foreground">
      <div
        className={cn(
          "desktop-entry-stage relative flex min-h-0 flex-1 flex-col overflow-hidden",
          HOME_PAGE_PADDING_CLASS,
        )}
      >
        {children ?? <Outlet />}
      </div>
    </main>
  );
}

export function DesktopEntryFallback() {
  const { t } = useI18n();

  return (
    <DesktopEntryLayout>
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
    </DesktopEntryLayout>
  );
}
