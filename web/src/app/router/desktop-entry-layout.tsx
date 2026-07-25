import { ReactNode } from "react";
import { Outlet } from "react-router-dom";

import { HOME_PAGE_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn } from "@/shared/ui/class-name";

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
  return (
    <DesktopEntryLayout>
      <div className="flex h-full items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
      </div>
    </DesktopEntryLayout>
  );
}
