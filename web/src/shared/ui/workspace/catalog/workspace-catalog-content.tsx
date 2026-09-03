import type {
  ElementType,
  HTMLAttributes,
  ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiTypographyClassName,
  type UiTypographyRole,
} from "@/shared/ui/typography/typography-styles";

type CatalogCardAlign = "start" | "center";
type CatalogFooterJustify = "between" | "start" | "end" | "center";
type CatalogTitleSize = "sm" | "md" | "lg";
type CatalogDescriptionSize = "sm" | "md";

const HEADER_ALIGN_CLASSES: Record<CatalogCardAlign, string> = {
  start: "flex items-start gap-3",
  center: "flex flex-col items-center gap-3 text-center",
};
const FOOTER_JUSTIFY_CLASSES: Record<CatalogFooterJustify, string> = {
  between: "justify-between",
  start: "justify-start",
  end: "justify-end",
  center: "justify-center",
};
const TITLE_ROLES: Record<CatalogTitleSize, UiTypographyRole> = {
  sm: "sectionTitle",
  md: "pageTitle",
  lg: "objectTitle",
};
const DESCRIPTION_ROLES: Record<CatalogDescriptionSize, UiTypographyRole> = {
  sm: "supporting",
  md: "body",
};
const LINE_CLAMP_CLASSES = {
  1: "line-clamp-1",
  2: "line-clamp-2",
  3: "line-clamp-3",
} as const;

export function WorkspaceCatalogHeader({
  children,
  className,
  align = "start",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  align?: CatalogCardAlign;
}) {
  return <div className={cn(HEADER_ALIGN_CLASSES[align], className)} {...props}>{children}</div>;
}

export function WorkspaceCatalogBody({
  children,
  className,
  grow = false,
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  grow?: boolean;
}) {
  return <div className={cn("mt-2.5", grow && "flex-1", className)} {...props}>{children}</div>;
}

export function WorkspaceCatalogFooter({
  children,
  className,
  justify = "between",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  justify?: CatalogFooterJustify;
}) {
  return (
    <div
      className={cn(
        "mt-3 flex min-h-[32px] items-end gap-3",
        FOOTER_JUSTIFY_CLASSES[justify],
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function WorkspaceCatalogTitle({
  children,
  as,
  className,
  size = "md",
  truncate = false,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  as?: ElementType;
  size?: CatalogTitleSize;
  truncate?: boolean;
}) {
  const Component = as ?? "h3";
  return (
    <Component
      className={cn(
        getUiTypographyClassName({ role: TITLE_ROLES[size], tone: "strong" }),
        truncate && "truncate",
        className,
      )}
      {...props}
    >
      {children}
    </Component>
  );
}

export function WorkspaceCatalogDescription({
  children,
  className,
  lines = 2,
  minHeight = false,
  size = "sm",
  ...props
}: HTMLAttributes<HTMLParagraphElement> & {
  children: ReactNode;
  lines?: 1 | 2 | 3;
  minHeight?: boolean;
  size?: CatalogDescriptionSize;
}) {
  return (
    <p
      className={cn(
        getUiTypographyClassName({ role: DESCRIPTION_ROLES[size], tone: "default" }),
        LINE_CLAMP_CLASSES[lines],
        minHeight && lines === 2 && "min-h-[40px]",
        className,
      )}
      {...props}
    >
      {children}
    </p>
  );
}
