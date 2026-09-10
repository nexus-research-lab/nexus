// INPUT: Workspace 内容、标题、滚动策略与 可选 mobile Header 语义。
// OUTPUT: 共享 Header、内容宽度和滚动骨架组成的 Workspace Surface。
// POS: Workspace 视图布局原语；不拥有业务资源、导航或动作生命周期。

import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME,
  MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME,
} from "@/shared/ui/layout/mobile-shell-header-layout";
import { WORKSPACE_CONTENT_GUTTER_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { WorkspaceSurfaceScaffold } from "./workspace-surface-scaffold";

interface WorkspaceSurfaceMobileHeaderConfig {
  action?: ReactNode;
  kind: "mobile";
  leading?: ReactNode;
}

interface WorkspaceSurfaceViewProps {
  title: string;
  header?: WorkspaceSurfaceMobileHeaderConfig;
  children: ReactNode;
  bodyScrollable?: boolean;
  /** 这里只允许滚动区和内容宽度的布局调整，不承担视觉覆写。 */
  bodyClassName?: string;
  contentClassName?: string;
  maxWidthClassName?: string;
}

export function WorkspaceSurfaceView({
  title,
  header,
  children,
  bodyScrollable = true,
  bodyClassName,
  contentClassName,
  maxWidthClassName = "max-w-[760px]",
}: WorkspaceSurfaceViewProps) {
  return (
    <WorkspaceSurfaceScaffold
      bodyClassName={cn(
        WORKSPACE_CONTENT_GUTTER_CLASS_NAME,
        "py-4",
        bodyClassName,
      )}
      bodyScrollable={bodyScrollable}
      header={header ? <WorkspaceSurfaceMobileHeader header={header} title={title} /> : undefined}
      stableGutter
    >
      <WorkspaceSurfaceContent
        className={cn(maxWidthClassName, contentClassName)}
        header={header}
        title={title}
      >
        {children}
      </WorkspaceSurfaceContent>
    </WorkspaceSurfaceScaffold>
  );
}

function WorkspaceSurfaceMobileHeader({
  header,
  title,
}: {
  header: WorkspaceSurfaceMobileHeaderConfig;
  title: string;
}) {
  return (
    <header
      className={cn(
        "shell-region-header flex shrink-0 items-center gap-2 border-b divider-subtle",
        MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME,
        MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME,
      )}
      data-desktop-window-controls-leading
      data-desktop-window-drag-region
    >
      {header.leading}
      <h2 className={cn(
        "min-w-0 flex-1 truncate",
        getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
      )}>
        {title}
      </h2>
      {header.action}
    </header>
  );
}

function WorkspaceSurfaceContent({
  children,
  className,
  header,
  title,
}: {
  children: ReactNode;
  className: string;
  header?: WorkspaceSurfaceMobileHeaderConfig;
  title: string;
}) {
  return (
    <div className={cn("mx-auto w-full", className)}>
      {!header ? (
        <h2 className="sr-only">{title}</h2>
      ) : null}
      {children}
    </div>
  );
}
