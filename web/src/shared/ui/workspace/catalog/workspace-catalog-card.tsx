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
  sm: "h-9 w-9 rounded-[14px]",
  md: "h-11 w-11 rounded-[16px]",
  lg: "h-14 w-14 rounded-[20px]",
};

const CATALOG_CARD_SIZE_CLASS_MAP: Record<CatalogCardSize, string> = {
  compact: "min-h-[138px] rounded-[20px] px-4 py-4",
  catalog: "min-h-[170px] rounded-[22px] px-5 py-4",
  comfort: "rounded-[26px] px-6 py-6",
  panel: "rounded-[30px] px-5 py-5 sm:px-6 sm:py-6",
  hero: "rounded-[32px] px-6 py-7 sm:px-8 sm:py-8",
  stat: "rounded-[24px] px-4 py-4",
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
  class_name,
  muted = false,
  size = "catalog",
  align = "start",
  interactive,
  onClick,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  class_name?: string;
  muted?: boolean;
  size?: CatalogCardSize;
  align?: CatalogCardAlign;
  interactive?: boolean;
}) {
  const is_interactive = interactive ?? Boolean(onClick);

  return (
    <article
      className={cn(
        "surface-card flex flex-col transition duration-(--motion-duration-fast) ease-out",
        CATALOG_CARD_SIZE_CLASS_MAP[size],
        align === "center" && "items-center text-center",
        is_interactive && "cursor-pointer hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        muted && "opacity-70",
        class_name,
      )}
      onClick={onClick}
      {...props}
    >
      {children}
    </article>
  );
}

export function WorkspaceCatalogHeader({
  children,
  class_name,
  align = "start",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  class_name?: string;
  align?: CatalogCardAlign;
}) {
  return (
    <div
      className={cn(CATALOG_HEADER_ALIGN_CLASS_MAP[align], class_name)}
      {...props}
    >
      {children}
    </div>
  );
}

export function WorkspaceCatalogBody({
  children,
  class_name,
  grow = false,
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  class_name?: string;
  grow?: boolean;
}) {
  return (
    <div className={cn("mt-2.5", grow && "flex-1", class_name)} {...props}>
      {children}
    </div>
  );
}

export function WorkspaceCatalogFooter({
  children,
  class_name,
  justify = "between",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  class_name?: string;
  justify?: CatalogFooterJustify;
}) {
  return (
    <div
      className={cn(
        "mt-3 flex min-h-[32px] items-end gap-3",
        CATALOG_FOOTER_JUSTIFY_CLASS_MAP[justify],
        class_name,
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
  class_name,
  size = "md",
  truncate = false,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  as?: ElementType;
  class_name?: string;
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
        class_name,
      )}
      {...props}
    >
      {children}
    </Component>
  );
}

export function WorkspaceCatalogDescription({
  children,
  class_name,
  lines = 2,
  min_height = false,
  size = "sm",
  ...props
}: HTMLAttributes<HTMLParagraphElement> & {
  children: ReactNode;
  class_name?: string;
  lines?: 1 | 2 | 3;
  min_height?: boolean;
  size?: CatalogDescriptionSize;
}) {
  const line_clamp_class_name =
    lines === 1 ? "line-clamp-1" : lines === 3 ? "line-clamp-3" : "line-clamp-2";
  return (
    <p
      className={cn(
        CATALOG_DESCRIPTION_CLASS_MAP[size],
        "text-(--text-default)",
        line_clamp_class_name,
        min_height && lines === 2 && "min-h-[40px]",
        class_name,
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
  class_name,
  shape = "rounded",
  size = "md",
  tone = "default",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  class_name?: string;
  shape?: CatalogMediaShape;
  size?: IconFrameSize;
  tone?: IconFrameTone;
}) {
  const tone_style = tone === "default" ? undefined : ICON_FRAME_TONE_STYLE_MAP[tone];

  return (
    <div
      className={cn(
        "chip-default flex shrink-0 items-center justify-center border",
        ICON_FRAME_SIZE_CLASS_MAP[size],
        ICON_FRAME_TONE_CLASS_MAP[tone],
        shape === "round" && "rounded-full",
        class_name,
      )}
      style={tone_style}
      {...props}
    >
      {children}
    </div>
  );
}

export function WorkspaceCatalogAction({
  children,
  class_name,
  tone = "default",
  size = "md",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  class_name?: string;
  tone?: CatalogActionTone;
  size?: CatalogActionSize;
}) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center rounded-[10px] border border-transparent bg-transparent text-(--icon-default) transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
        size === "sm"
          ? "h-6 w-6 rounded-[8px]"
          : "h-8 w-8 rounded-[10px]",
        tone === "danger" && "hover:border-rose-100 hover:bg-rose-50/92 hover:text-rose-600/90",
        class_name,
      )}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
}

export function WorkspaceCatalogTextAction({
  children,
  class_name,
  tone = "default",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  class_name?: string;
  tone?: CatalogTextActionTone;
}) {
  return (
    <button
      className={cn(
        "inline-flex items-center gap-1.5 text-xs font-semibold transition duration-(--motion-duration-fast) ease-out",
        tone === "default" && "text-(--text-default) hover:text-(--text-strong)",
        tone === "primary" && "text-(--primary) hover:text-[color:color-mix(in_srgb,var(--primary)_86%,var(--foreground)_14%)]",
        tone === "danger" && "text-rose-600/88 hover:text-rose-700",
        class_name,
      )}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
}

export function WorkspaceCatalogTag({
  children,
  class_name,
}: {
  children: ReactNode;
  class_name?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded-full px-2 text-2xs font-medium leading-none text-(--text-default)",
        class_name,
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
  class_name,
  size = "comfort",
  onClick,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  class_name?: string;
  size?: Extract<CatalogCardSize, "compact" | "catalog" | "comfort" | "panel">;
}) {
  return (
    <article
      className={cn(
        "surface-card flex flex-col items-center justify-center border border-dashed border-(--surface-panel-subtle-border) text-center transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        CATALOG_CARD_SIZE_CLASS_MAP[size],
        onClick && "cursor-pointer",
        class_name,
      )}
      onClick={onClick}
      {...props}
    >
      {children}
    </article>
  );
}

