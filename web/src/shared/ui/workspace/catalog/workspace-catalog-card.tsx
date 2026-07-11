"use client";

import type {
  HTMLAttributes,
  KeyboardEvent,
  ReactNode,
} from "react";

import { cn } from "@/lib/utils";

type CatalogCardSize = "compact" | "catalog" | "comfort" | "panel" | "hero" | "stat";
type CatalogCardAlign = "start" | "center";

const CATALOG_CARD_SIZE_CLASSES: Record<CatalogCardSize, string> = {
  compact: "min-h-[138px] rounded-[12px] px-4 py-4",
  catalog: "min-h-[170px] rounded-[12px] px-5 py-4",
  comfort: "rounded-[14px] px-6 py-6",
  panel: "rounded-[14px] px-5 py-5 sm:px-6 sm:py-6",
  hero: "rounded-[14px] px-6 py-7 sm:px-8 sm:py-8",
  stat: "rounded-[12px] px-4 py-4",
};

function activateCardFromKeyboard(event: KeyboardEvent<HTMLElement>): void {
  if (event.key !== "Enter" && event.key !== " ") {
    return;
  }
  event.preventDefault();
  event.currentTarget.click();
}

export function WorkspaceCatalogCard({
  children,
  className,
  muted = false,
  size = "catalog",
  align = "start",
  interactive,
  onClick,
  onKeyDown,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  muted?: boolean;
  size?: CatalogCardSize;
  align?: CatalogCardAlign;
  interactive?: boolean;
}) {
  const isInteractive = interactive ?? Boolean(onClick);
  return (
    // role 和键盘协议只在交互卡片启用，卡片内部仍可容纳独立动作按钮。
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions
    <article
      className={cn(
        "flex flex-col border border-(--divider-subtle-color) bg-transparent transition duration-(--motion-duration-fast) ease-out",
        CATALOG_CARD_SIZE_CLASSES[size],
        align === "center" && "items-center text-center",
        isInteractive && "cursor-pointer hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        muted && "opacity-70",
        className,
      )}
      onClick={onClick}
      onKeyDown={(event) => {
        onKeyDown?.(event);
        if (isInteractive && !event.defaultPrevented) {
          activateCardFromKeyboard(event);
        }
      }}
      role={isInteractive ? "button" : undefined}
      tabIndex={isInteractive ? 0 : undefined}
      {...props}
    >
      {children}
    </article>
  );
}

export function WorkspaceCatalogGhostCard({
  children,
  className,
  size = "comfort",
  onClick,
  onKeyDown,
  ...props
}: HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  size?: Extract<CatalogCardSize, "compact" | "catalog" | "comfort" | "panel">;
}) {
  return (
    // Ghost Card 只在提供命令时进入键盘序列，静态占位不伪装成按钮。
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions
    <article
      className={cn(
        "flex flex-col items-center justify-center border border-dashed border-(--divider-subtle-color) bg-transparent text-center transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        CATALOG_CARD_SIZE_CLASSES[size],
        onClick && "cursor-pointer",
        className,
      )}
      onClick={onClick}
      onKeyDown={(event) => {
        onKeyDown?.(event);
        if (onClick && !event.defaultPrevented) {
          activateCardFromKeyboard(event);
        }
      }}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
      {...props}
    >
      {children}
    </article>
  );
}
