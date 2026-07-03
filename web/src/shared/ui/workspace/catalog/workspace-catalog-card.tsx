/**
 * =====================================================
 * @File   : workspace-catalog-card.tsx
 * @Date   : 2026-04-05 14:32
 * @Author : leemysw
 * 2026-04-05 14:32   Create
 * =====================================================
 */

"use client";

import { ButtonHTMLAttributes, CSSProperties, ElementType, HTMLAttributes, ReactNode } from "react";

import { cn } from "@/lib/utils";
import { UiButton } from "@/shared/ui/button";
import { UiListActionButton } from "@/shared/ui/list-action";

type CatalogMediaShape = "round" | "rounded";
type CatalogActionTone = "default" | "danger";
type CatalogActionSize = "sm" | "md";
type CatalogTextActionTone = "default" | "primary" | "danger";
type CatalogCardSize = "compact" | "catalog" | "comfort" | "panel" | "hero" | "stat";
type CatalogCardAlign = "start" | "center";
type CatalogFooterJustify = "between" | "start" | "end" | "center";
type CatalogTitleSize = "sm" | "md" | "lg" | "hero";
type CatalogDescriptionSize = "sm" | "md";
type IconFrameTone = "default" | "primary" | "success" | "warning";
type IconFrameSize = "sm" | "md" | "lg";

const ICON_FRAME_TONE_CLASS_MAP: Record<IconFrameTone, string> = {
  default: "border-transparent bg-(--chip-default-background) text-(--text-default)",
  primary: "",
  success: "",
  warning: "",
};

const ICON_FRAME_TONE_STYLE_MAP: Record<Exclude<IconFrameTone, "default">, CSSProperties> = {
  primary: {
    background: "color-mix(in srgb, var(--primary) 14%, var(--chip-default-background))",
    border: "1px solid color-mix(in srgb, var(--primary) 32%, var(--chip-default-border))",
    color: "color-mix(in srgb, var(--primary) 88%, var(--text-strong))",
  },
  success: {
    background: "color-mix(in srgb, var(--success) 16%, var(--chip-default-background))",
    border: "1px solid color-mix(in srgb, var(--success) 32%, var(--chip-default-border))",
    color: "color-mix(in srgb, var(--success) 84%, var(--text-strong))",
  },
  warning: {
    background: "color-mix(in srgb, var(--warning) 16%, var(--chip-default-background))",
    border: "1px solid color-mix(in srgb, var(--warning) 34%, var(--chip-default-border))",
    color: "color-mix(in srgb, var(--warning) 84%, var(--text-strong))",
  },
};

const ICON_FRAME_SIZE_CLASS_MAP: Record<IconFrameSize, string> = {
  sm: "h-9 w-9 rounded-[10px]",
  md: "h-11 w-11 rounded-[12px]",
  lg: "h-14 w-14 rounded-[14px]",
};

const CATALOG_CARD_SIZE_CLASS_MAP: Record<CatalogCardSize, string> = {
  compact: "min-h-[138px] rounded-[12px] px-4 py-4",
  catalog: "min-h-[170px] rounded-[12px] px-5 py-4",
  comfort: "rounded-[14px] px-6 py-6",
  panel: "rounded-[14px] px-5 py-5 sm:px-6 sm:py-6",
  hero: "rounded-[14px] px-6 py-7 sm:px-8 sm:py-8",
  stat: "rounded-[12px] px-4 py-4",
};

const CATALOG_HEADER_ALIGN_CLASS_MAP: Record<CatalogCardAlign, string> = {
  start: "flex items-start gap-3",
  center: "flex flex-col items-center gap-3 text-center",
};

const CATALOG_FOOTER_JUSTIFY_CLASS_MAP: Record<CatalogFooterJustify, string> = {
  between: "justify-between",
  start: "justify-start",
  end: "justify-end",
  center: "justify-center",
};

const CATALOG_TITLE_CLASS_MAP: Record<CatalogTitleSize, string> = {
  sm: "text-base font-semibold tracking-[-0.02em]",
  md: "text-md font-bold tracking-[-0.04em]",
  lg: "text-lg font-bold tracking-[-0.03em]",
  hero: "text-[clamp(2rem,4.6vw,3.4rem)] font-black leading-[0.94] tracking-[-0.06em]",
};

const CATALOG_DESCRIPTION_CLASS_MAP: Record<CatalogDescriptionSize, string> = {
  sm: "text-sm leading-[1.55]",
  md: "text-base leading-8",
};

/** 中文注释：这组目录卡片是高频共享块，长相收回组件层，避免全局 CSS 继续膨胀。 */
export function WorkspaceCatalogCard({
  children,
  className: className,
  muted = false,
  size = "catalog",
  align = "start",
  interactive,
  onClick,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  className?: string;
  muted?: boolean;
  size?: CatalogCardSize;
  align?: CatalogCardAlign;
  interactive?: boolean;
}) {
  const isInteractive = interactive ?? Boolean(onClick);

  return (
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- role="button" + tabIndex are set at runtime when interactive
    <article
      className={cn(
        "flex flex-col border border-(--divider-subtle-color) bg-transparent transition duration-(--motion-duration-fast) ease-out",
        CATALOG_CARD_SIZE_CLASS_MAP[size],
        align === "center" && "items-center text-center",
        isInteractive && "cursor-pointer hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        muted && "opacity-70",
        className,
      )}
      onClick={onClick}
      onKeyDown={isInteractive ? (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          (event.currentTarget as HTMLElement).click();
        }
      } : undefined}
      role={isInteractive ? "button" : undefined}
      tabIndex={isInteractive ? 0 : undefined}
      {...props}
    >
      {children}
    </article>
  );
}

export function WorkspaceCatalogHeader({
  children,
  className: className,
  align = "start",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  className?: string;
  align?: CatalogCardAlign;
}) {
  return (
    <div
      className={cn(CATALOG_HEADER_ALIGN_CLASS_MAP[align], className)}
      {...props}
    >
      {children}
    </div>
  );
}

export function WorkspaceCatalogBody({
  children,
  className: className,
  grow = false,
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  className?: string;
  grow?: boolean;
}) {
  return (
    <div className={cn("mt-2.5", grow && "flex-1", className)} {...props}>
      {children}
    </div>
  );
}

export function WorkspaceCatalogFooter({
  children,
  className: className,
  justify = "between",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  className?: string;
  justify?: CatalogFooterJustify;
}) {
  return (
    <div
      className={cn(
        "mt-3 flex min-h-[32px] items-end gap-3",
        CATALOG_FOOTER_JUSTIFY_CLASS_MAP[justify],
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
  className: className,
  size = "md",
  truncate = false,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  as?: ElementType;
  className?: string;
  size?: CatalogTitleSize;
  truncate?: boolean;
}) {
  const Component = as ?? "h3";
  return (
    <Component
      className={cn(
        CATALOG_TITLE_CLASS_MAP[size],
        "text-(--text-strong)",
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
  className: className,
  lines = 2,
  minHeight: minHeight = false,
  size = "sm",
  ...props
}: HTMLAttributes<HTMLParagraphElement> & {
  children: ReactNode;
  className?: string;
  lines?: 1 | 2 | 3;
  minHeight?: boolean;
  size?: CatalogDescriptionSize;
}) {
  const lineClampClassName =
    lines === 1 ? "line-clamp-1" : lines === 3 ? "line-clamp-3" : "line-clamp-2";
  return (
    <p
      className={cn(
        CATALOG_DESCRIPTION_CLASS_MAP[size],
        "text-(--text-default)",
        lineClampClassName,
        minHeight && lines === 2 && "min-h-[40px]",
        className,
      )}
      {...props}
    >
      {children}
    </p>
  );
}

/** 中文注释：统一高频图标容器，侧栏、卡片和弹窗都用这套边界语法。 */
export function WorkspaceIconFrame({
  children,
  className: className,
  shape = "rounded",
  size = "md",
  tone = "default",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  className?: string;
  shape?: CatalogMediaShape;
  size?: IconFrameSize;
  tone?: IconFrameTone;
}) {
  const toneStyle = tone === "default" ? undefined : ICON_FRAME_TONE_STYLE_MAP[tone];

  return (
    <div
      className={cn(
        "chip-default flex shrink-0 items-center justify-center border",
        ICON_FRAME_SIZE_CLASS_MAP[size],
        ICON_FRAME_TONE_CLASS_MAP[tone],
        shape === "round" && "rounded-full",
        className,
      )}
      style={toneStyle}
      {...props}
    >
      {children}
    </div>
  );
}

export function WorkspaceCatalogAction({
  children,
  className,
  tone = "default",
  size = "md",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  className?: string;
  tone?: CatalogActionTone;
  size?: CatalogActionSize;
}) {
  return (
    <UiListActionButton
      className={className}
      size={size === "sm" ? "xs" : "md"}
      tone={tone}
      type={type}
      visibility="visible"
      {...props}
    >
      {children}
    </UiListActionButton>
  );
}

export function WorkspaceCatalogTextAction({
  children,
  className,
  tone = "default",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  className?: string;
  tone?: CatalogTextActionTone;
}) {
  return (
    <UiButton
      className={className}
      size="sm"
      tone={tone}
      type={type}
      variant="text"
      {...props}
    >
      {children}
    </UiButton>
  );
}

export function WorkspaceCatalogTag({
  children,
  className: className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded-full px-2 text-2xs font-medium leading-none text-(--text-default)",
        className,
      )}
      style={{
        background: "color-mix(in srgb, var(--surface-panel-subtle-background) 58%, transparent)",
        border: "1px solid color-mix(in srgb, var(--surface-panel-subtle-border) 72%, transparent)",
      }}
    >
      {children}
    </span>
  );
}

export function WorkspaceCatalogGhostCard({
  children,
  className: className,
  size = "comfort",
  onClick,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  className?: string;
  size?: Extract<CatalogCardSize, "compact" | "catalog" | "comfort" | "panel">;
}) {
  return (
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- role="button" + tabIndex are set at runtime when onClick is provided
    <article
      className={cn(
        "flex flex-col items-center justify-center border border-dashed border-(--divider-subtle-color) bg-transparent text-center transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        CATALOG_CARD_SIZE_CLASS_MAP[size],
        onClick && "cursor-pointer",
        className,
      )}
      onClick={onClick}
      onKeyDown={onClick ? (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          (event.currentTarget as HTMLElement).click();
        }
      } : undefined}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      {...props}
    >
      {children}
    </article>
  );
}
