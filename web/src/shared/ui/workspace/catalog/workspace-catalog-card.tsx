// INPUT: 目录内容、密度、对齐与可选整卡主动作。
// OUTPUT: 保留 article 语义的卡片、独立主动作，以及复用 Button 状态的目录创建入口。
// POS: 跨领域目录密度、圆角与覆盖命中几何所有者；业务权限和命令由调用方决定。

import type {
  ButtonHTMLAttributes,
  HTMLAttributes,
  ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import "./workspace-catalog-card.css";

type CatalogCardSize = "dense" | "compact" | "catalog" | "comfort" | "panel";
type CatalogCardAlign = "start" | "center";

const CATALOG_CARD_SIZE_CLASSES: Record<CatalogCardSize, string> = {
  dense: "min-h-[104px] px-3.5 py-3",
  compact: "min-h-[138px] px-4 py-4",
  catalog: "min-h-[170px] px-5 py-4",
  comfort: "px-6 py-6",
  panel: "px-5 py-5 sm:px-6 sm:py-6",
};

const CATALOG_CARD_RADIUS_CLASSES: Record<CatalogCardSize, string> = {
  dense: "radius-control-md",
  compact: "surface-radius-md",
  catalog: "surface-radius-md",
  comfort: "surface-radius-lg",
  panel: "surface-radius-lg",
};

export function WorkspaceCatalogCard({
  children,
  className,
  muted = false,
  primaryAction,
  size = "catalog",
  align = "start",
  ...props
}: Omit<
  HTMLAttributes<HTMLElement>,
  "onClick" | "onKeyDown" | "role" | "tabIndex"
> & {
  children: ReactNode;
  muted?: boolean;
  primaryAction?: {
    disabled?: boolean;
    label: string;
    onClick: NonNullable<ButtonHTMLAttributes<HTMLButtonElement>["onClick"]>;
  };
  size?: CatalogCardSize;
  align?: CatalogCardAlign;
}) {
  return (
    <article
      className={cn(
        "flex flex-col border border-(--divider-subtle-color) bg-transparent transition duration-(--motion-duration-fast) ease-out",
        CATALOG_CARD_SIZE_CLASSES[size],
        CATALOG_CARD_RADIUS_CLASSES[size],
        align === "center" && "items-center text-center",
        primaryAction && "workspace-catalog-card-action-surface",
        primaryAction && !primaryAction.disabled && "hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        muted && "opacity-70",
        className,
      )}
      {...props}
    >
      {primaryAction ? (
        <UiButton
          aria-label={primaryAction.label}
          className={cn(
            "absolute inset-0 -z-1 h-full min-h-0 w-full border-0 p-0 focus-visible:ring-inset",
            CATALOG_CARD_RADIUS_CLASSES[size],
          )}
          data-slot="catalog-primary-action"
          disabled={primaryAction.disabled}
          onClick={primaryAction.onClick}
          variant="ghost"
        >
          <span className="sr-only">{primaryAction.label}</span>
        </UiButton>
      ) : null}
      {children}
    </article>
  );
}

export function WorkspaceCatalogGhostAction({
  children,
  className,
  size = "comfort",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  size?: Extract<CatalogCardSize, "compact" | "catalog" | "comfort" | "panel">;
}) {
  return (
    <UiButton
      className={cn(
        "flex h-auto w-full flex-col items-center justify-center border-dashed text-center",
        CATALOG_CARD_SIZE_CLASSES[size],
        CATALOG_CARD_RADIUS_CLASSES[size],
        className,
      )}
      type={type}
      variant="outline"
      {...props}
    >
      {children}
    </UiButton>
  );
}
