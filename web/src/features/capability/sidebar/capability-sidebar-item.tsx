import { cn } from "@/shared/ui/class-name";
import { UiListRow } from "@/shared/ui/list/list-row";

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
  const handleClick = () => {
    onSelect(item);
  };

  return (
    <UiListRow
      active={active}
      activeTone="sidebar"
      className="min-h-[54px] gap-2.5 rounded-[8px] px-2 py-1.5 max-lg:min-h-[72px] max-lg:gap-3 max-lg:rounded-[12px] max-lg:px-3 max-lg:py-2.5"
      leading={(
        <span className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center radius-control-sm border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_55%,transparent)] text-(--icon-muted) max-lg:h-10 max-lg:w-10 max-lg:rounded-[10px]",
          active && "border-(--divider-strong-color) bg-(--surface-interactive-hover-background) text-(--icon-strong)",
        )}>
          <Icon className="h-4 w-4 max-lg:h-[18px] max-lg:w-[18px]" />
        </span>
      )}
      onClick={handleClick}
      right={(
        <span className={cn(
          "shrink-0 text-xs font-medium tabular-nums text-(--text-soft)",
          active && "text-(--text-muted)",
        )}>
          {item.meta}
        </span>
      )}
      title={item.label}
    />
  );
}
