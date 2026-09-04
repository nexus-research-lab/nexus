// INPUT: 侧栏导航项图标、标签、选中态与原生按钮交互属性。
// OUTPUT: 一级入口和固定会话共用的 Dock 动作、图标框、文字与计数结构。
// POS: 宽侧栏导航轨的唯一动作 DOM/视觉所有者；不判断路由、排序或业务计数。

import type { LucideIcon } from "lucide-react";
import type { ButtonHTMLAttributes } from "react";

import { UiCounterBadge } from "@/shared/ui/display/badge";
import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

type SidebarRailActionLayout = "primary" | "pinned";

interface SidebarRailActionProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children"
> {
  active: boolean;
  badgeCount?: number;
  icon: LucideIcon;
  label: string;
  layout: SidebarRailActionLayout;
  supplementalLabel?: string;
}

const BUTTON_LAYOUT_CLASS_NAMES: Record<SidebarRailActionLayout, string> = {
  primary:
    "flex h-[50px] w-10 flex-col items-center justify-center gap-0.5 rounded-[12px]",
  pinned:
    "absolute inset-0 cursor-grab rounded-[12px] active:cursor-grabbing",
};

const ICON_FRAME_LAYOUT_CLASS_NAMES: Record<SidebarRailActionLayout, string> = {
  primary: "relative shrink-0",
  pinned: "absolute left-1/2 top-0 -translate-x-1/2",
};

const LABEL_LAYOUT_CLASS_NAMES: Record<SidebarRailActionLayout, string> = {
  primary: "max-w-full truncate px-1 leading-tight",
  pinned:
    "absolute inset-x-0 bottom-2 block truncate px-1 text-center leading-tight",
};

export function SidebarRailAction({
  active,
  badgeCount = 0,
  className,
  icon: Icon,
  label,
  layout,
  supplementalLabel,
  type = "button",
  ...props
}: SidebarRailActionProps) {
  return (
    <button
      aria-current={active ? "page" : undefined}
      aria-pressed={layout === "primary" ? active : undefined}
      className={cn(
        "group/sidebar-rail-action relative min-w-0 border-0 bg-transparent p-0 transition-colors duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_34%,transparent)]",
        getUiTypographyClassName({ role: "caption", weight: "medium" }),
        active
          ? "text-(--text-strong)"
          : "text-(--text-muted) hover:text-(--text-strong)",
        BUTTON_LAYOUT_CLASS_NAMES[layout],
        className,
      )}
      type={type}
      {...props}
    >
      <span
        className={cn(
          "flex h-8 w-8 items-center justify-center rounded-[10px] transition-[background,color] duration-(--motion-duration-fast)",
          ICON_FRAME_LAYOUT_CLASS_NAMES[layout],
          active
            ? SIDEBAR_SELECTION_CLASS_NAME
            : "group-hover/sidebar-rail-action:bg-(--surface-interactive-hover-background)",
        )}
      >
        <Icon className="h-[18px] w-[18px]" />
        <UiCounterBadge
          aria-hidden="true"
          className="absolute -right-1.5 -top-1.5 h-4 min-w-4 px-1 text-2xs"
          count={badgeCount}
        />
      </span>
      <span className={LABEL_LAYOUT_CLASS_NAMES[layout]}>{label}</span>
      {supplementalLabel ? <span className="sr-only">{supplementalLabel}</span> : null}
    </button>
  );
}
