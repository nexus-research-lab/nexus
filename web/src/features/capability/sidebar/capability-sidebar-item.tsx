// INPUT: 已投影的能力导航项、选中状态与选择命令。
// OUTPUT: 共享 ListRow 中的能力图标、名称和稳定计数。
// POS: 能力侧栏单行纯视图；不解释计数来源或执行路由。
import { cn } from "@/shared/ui/class-name";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
      className="min-h-14 gap-2.5 px-2 py-1.5 max-[559px]:min-h-18 max-[559px]:gap-3 max-[559px]:px-3 max-[559px]:py-2.5"
      leading={(
        <span className={cn(
          "flex h-8 w-8 shrink-0 items-center justify-center radius-control-md border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_55%,transparent)] text-(--icon-muted) max-[559px]:h-10 max-[559px]:w-10",
          active && "border-(--divider-strong-color) bg-(--surface-interactive-hover-background) text-(--icon-strong)",
        )}>
          <Icon className="h-4 w-4 max-[559px]:h-5 max-[559px]:w-5" />
        </span>
      )}
      onClick={handleClick}
      right={(
        <span className={cn(
          "shrink-0 tabular-nums",
          getUiTypographyClassName({
            role: "caption",
            tone: active ? "muted" : "soft",
            weight: "medium",
          }),
        )}>
          {item.meta}
        </span>
      )}
      title={item.label}
    />
  );
}
