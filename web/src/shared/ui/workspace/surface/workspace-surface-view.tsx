import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

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

type WorkspaceSurfaceViewHeader =
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
      bodyClassName={cn("px-4 py-4 sm:px-5 xl:px-6", bodyClassName)}
      bodyScrollable={bodyScrollable}
      header={header?.kind === "page" ? (
        <WorkspaceSurfacePageHeader
          header={header}
          maxWidthClassName={maxWidthClassName}
          title={title}
        />
      ) : undefined}
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
    <div className="px-5 py-2.5 xl:px-6">
      <div className={cn("mx-auto flex w-full items-center justify-between gap-3", maxWidthClassName)}>
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-[16px] font-semibold leading-5 tracking-normal text-(--text-strong)">
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
      {header?.kind !== "page" ? <h2 className="sr-only">{title}</h2> : null}
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
