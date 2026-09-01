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
      className="min-h-[54px] gap-2.5 rounded-[8px] px-2 py-1.5 max-[559px]:min-h-[72px] max-[559px]:gap-3 max-[559px]:rounded-[12px] max-[559px]:px-3 max-[559px]:py-2.5"
      leading={(
        <span className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center radius-control-sm border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_55%,transparent)] text-(--icon-muted) max-[559px]:h-10 max-[559px]:w-10 max-[559px]:rounded-[10px]",
          active && "border-(--divider-strong-color) bg-(--surface-interactive-hover-background) text-(--icon-strong)",
        )}>
          <Icon className="h-4 w-4 max-[559px]:h-[18px] max-[559px]:w-[18px]" />
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
