import type { MouseEventHandler } from "react";

import type { CapabilitySidebarItem } from "./capability-sidebar-model";

interface CapabilitySidebarItemViewProps {
  active: boolean;
  item: CapabilitySidebarItem;
  onSelect: (item: CapabilitySidebarItem) => void;
}

export function CapabilitySidebarItemView({
  active,
  item,
  onSelect,
}: CapabilitySidebarItemViewProps) {
  const Icon = item.icon;
  const handleClick: MouseEventHandler<HTMLButtonElement> = () => {
    onSelect(item);
  };

  return (
    <button
      className="group/item relative box-border flex w-full min-w-0 cursor-pointer items-center gap-2.5 rounded-[12px] px-2.5 py-[7px] text-left text-[14px] text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) data-[active=true]:bg-[color:color-mix(in_srgb,var(--surface-interactive-active-background)_72%,transparent)] data-[active=true]:font-medium data-[active=true]:text-(--text-strong)"
      data-active={String(active)}
      onClick={handleClick}
      type="button"
    >
      <span className="absolute left-0 top-1/2 h-4 w-[3px] -translate-y-1/2 rounded-full bg-(--primary) opacity-0 transition-opacity duration-(--motion-duration-fast) group-data-[active=true]/item:opacity-100" />
      <span className="flex h-6 w-6 shrink-0 items-center justify-center text-(--icon-muted) group-data-[active=true]/item:text-(--primary)">
        <Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1 truncate">{item.label}</span>
      <span className="shrink-0 text-[12px] font-medium tabular-nums text-(--text-soft) group-data-[active=true]/item:text-(--text-muted)">
        {item.meta}
      </span>
    </button>
  );
}
