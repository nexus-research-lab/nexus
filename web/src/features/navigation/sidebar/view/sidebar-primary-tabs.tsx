import { UiCounterBadge } from "@/shared/ui/display/badge";
import { cn } from "@/lib/utils";

import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
} from "./sidebar-wide-panel-types";

interface SidebarPrimaryTabsProps {
  activeTab: SidebarPrimaryTab;
  items: SidebarPrimaryTabItem[];
  onSelect: (tab: SidebarPrimaryTab) => void;
  variant: "rail" | "panel";
}

export function SidebarPrimaryTabs({
  activeTab,
  items,
  onSelect,
  variant,
}: SidebarPrimaryTabsProps) {
  const isRail = variant === "rail";
  return (
    <div
      className={
        isRail
          ? "mt-1 flex flex-col items-center gap-1.5"
          : "grid grid-cols-3 gap-1 rounded-[14px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)] p-1"
      }
    >
      {items.map((item) => (
        <PrimaryTabButton
          active={activeTab === item.key}
          item={item}
          key={item.key}
          onSelect={onSelect}
          variant={variant}
        />
      ))}
    </div>
  );
}

function PrimaryTabButton({
  active,
  item,
  onSelect,
  variant,
}: {
  active: boolean;
  item: SidebarPrimaryTabItem;
  onSelect: (tab: SidebarPrimaryTab) => void;
  variant: SidebarPrimaryTabsProps["variant"];
}) {
  const Icon = item.icon;
  const isRail = variant === "rail";
  return (
    <button
      aria-current={active ? "page" : undefined}
      aria-label={isRail ? item.label : undefined}
      aria-pressed={active}
      className={cn(
        isRail
          ? "relative flex h-9 w-9 items-center justify-center rounded-full text-(--icon-default) transition-(background,color,transform) duration-(--motion-duration-fast) hover:-translate-y-px hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
          : "flex h-9 items-center justify-center gap-1.5 rounded-[11px] text-[13px] font-medium transition-[background,color,box-shadow] duration-(--motion-duration-fast)",
        active && isRail &&
          "bg-(--surface-interactive-active-background) text-(--primary)",
        active && !isRail &&
          "bg-[color:color-mix(in_srgb,var(--primary)_14%,var(--surface-elevated-background))] text-(--primary) shadow-[0_8px_22px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
        !active && !isRail && "text-(--text-muted) hover:text-(--text-strong)",
      )}
      data-tour-anchor={item.anchor}
      onClick={() => onSelect(item.key)}
      title={item.label}
      type="button"
    >
      {isRail ? (
        <>
          <Icon
            className={cn(
              "h-4 w-4",
              active && "fill-(--primary) stroke-(--primary)",
            )}
          />
          <UiCounterBadge
            className="absolute -right-1 -top-1 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]"
            count={item.badgeCount}
          />
        </>
      ) : (
        <span className="relative flex h-4 w-4 items-center justify-center">
          <Icon
            className={cn(
              "h-3.5 w-3.5",
              active && "fill-(--primary) stroke-(--primary)",
            )}
          />
          <UiCounterBadge
            className="absolute -right-2.5 -top-2 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]"
            count={item.badgeCount}
          />
        </span>
      )}
      {!isRail ? <span>{item.label}</span> : null}
    </button>
  );
}
