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
      className="min-h-[54px] gap-2.5 rounded-[8px] px-2 py-1.5"
      leading={(
        <span className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center rounded-[9px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_55%,transparent)] text-(--icon-muted)",
          active && "border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color)_78%)] text-(--primary)",
        )}>
          <Icon className="h-4 w-4" />
        </span>
      )}
      onClick={handleClick}
      right={(
        <span className={cn(
          "shrink-0 text-[11px] font-medium tabular-nums text-(--text-soft)",
          active && "text-(--text-muted)",
        )}>
          {item.meta}
        </span>
      )}
      title={item.label}
    />
  );
}
