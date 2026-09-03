// INPUT: Workspace 内容、标题、滚动策略与 page/mobile/overlay Header 语义。
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

interface WorkspaceSurfacePageHeader {
  action?: ReactNode;
  kind: "page";
}

interface WorkspaceSurfaceOverlayHeader {
  action?: ReactNode;
  kind: "overlay";
  leading: ReactNode;
}

interface WorkspaceSurfaceMobileHeaderConfig {
  action?: ReactNode;
  kind: "mobile";
  leading?: ReactNode;
}

type WorkspaceSurfaceViewHeader =
  | WorkspaceSurfaceMobileHeaderConfig
  | WorkspaceSurfaceOverlayHeader
  | WorkspaceSurfacePageHeader;

interface WorkspaceSurfaceViewProps {
  title: string;
  header?: WorkspaceSurfaceViewHeader;
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
      header={resolveWorkspaceSurfaceHeader(header, maxWidthClassName, title)}
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

function resolveWorkspaceSurfaceHeader(
  header: WorkspaceSurfaceViewHeader | undefined,
  maxWidthClassName: string,
  title: string,
): ReactNode {
  if (header?.kind === "page") {
    return (
      <WorkspaceSurfacePageHeader
        header={header}
        maxWidthClassName={maxWidthClassName}
        title={title}
      />
    );
  }
  if (header?.kind === "mobile") {
    return <WorkspaceSurfaceMobileHeader header={header} title={title} />;
  }
  return undefined;
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

function WorkspaceSurfacePageHeader({
  header,
  maxWidthClassName,
  title,
}: {
  header: WorkspaceSurfacePageHeader;
  maxWidthClassName: string;
  title: string;
}) {
  return (
    <div className={cn(WORKSPACE_CONTENT_GUTTER_CLASS_NAME, "py-2.5")}>
      <div className={cn("mx-auto flex w-full items-center justify-between gap-3", maxWidthClassName)}>
        <div className="min-w-0 flex-1">
          <h2 className={cn(
            "truncate",
            getUiTypographyClassName({ role: "pageTitle", tone: "strong" }),
          )}>
            {title}
          </h2>
        </div>
        {header.action}
      </div>
      <div className={cn("mx-auto mt-2 w-full", maxWidthClassName)}>
        <div className="h-px w-full rounded-full bg-(--divider-subtle-color)" />
      </div>
    </div>
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
  header?: WorkspaceSurfaceViewHeader;
  title: string;
}) {
  return (
    <div className={cn("mx-auto w-full", className)}>
      {header?.kind !== "page" && header?.kind !== "mobile" ? (
        <h2 className="sr-only">{title}</h2>
      ) : null}
      {header?.kind === "overlay" ? (
        <div className="sticky top-0 z-20 flex h-7 shrink-0 items-start justify-between gap-3">
          <div className="min-w-0 flex-1 text-(--text-default)">
            {header.leading}
          </div>
          {header.action}
        </div>
      ) : null}
      {children}
    </div>
  );
}
