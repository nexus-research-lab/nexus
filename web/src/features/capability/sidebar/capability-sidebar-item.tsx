// INPUT: 已投影的能力导航项、选中状态与选择命令。
// OUTPUT: 共享 ListRow 中的能力图标与名称；不显示数量。
// POS: 能力侧栏单行纯视图；不执行路由。
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
      className="min-h-10 gap-2.5 px-2.5 py-1.5 max-[559px]:min-h-12 max-[559px]:gap-3 max-[559px]:px-3 max-[559px]:py-2"
      leading={(
        // 图标直接跟随行文字，不再包一层描边方块；当前项只提升图标颜色。
        <Icon
          aria-hidden
          className={cn(
            "h-4 w-4 shrink-0 text-(--icon-muted) max-[559px]:h-5 max-[559px]:w-5",
            active && "text-(--icon-strong)",
          )}
        />
      )}
      onClick={handleClick}
      title={item.label}
    />
  );
}
