// INPUT: 目录内容、密度、对齐与可选整卡主动作。
// OUTPUT: 保留 article 语义的卡片，以及与内容内独立动作互不嵌套的原生主按钮。
// POS: 跨领域目录卡片与覆盖命中几何所有者；业务权限和命令由调用方决定。

import type {
  ButtonHTMLAttributes,
  HTMLAttributes,
  ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import "./workspace-catalog-card.css";

type CatalogCardSize = "compact" | "catalog" | "comfort" | "panel";
type CatalogCardAlign = "start" | "center";

const CATALOG_CARD_SIZE_CLASSES: Record<CatalogCardSize, string> = {
  compact: "min-h-[138px] surface-radius-md px-4 py-4",
  catalog: "min-h-[170px] surface-radius-md px-5 py-4",
  comfort: "surface-radius-lg px-6 py-6",
  panel: "surface-radius-lg px-5 py-5 sm:px-6 sm:py-6",
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
            size === "compact" || size === "catalog" ? "surface-radius-md" : "surface-radius-lg",
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
    <button
      className={cn(
        "flex w-full cursor-pointer flex-col items-center justify-center border border-dashed border-(--divider-subtle-color) bg-transparent text-center transition duration-(--motion-duration-fast) ease-out hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)",
        CATALOG_CARD_SIZE_CLASSES[size],
        className,
      )}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
}
