// INPUT: 当前选中项与路由导航命令。
// OUTPUT: 使用共享列表行的能力目录侧栏；六个固定入口一眼可见，不提供搜索。
// POS: 能力导航装配层；单行呈现归相邻 model/view。

import { memo, useCallback, useMemo } from "react";
import { useNavigate } from "react-router-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useScrollFade } from "@/shared/lib/react/use-scroll-fade";
import { useSidebarStore } from "@/store/sidebar";

import { CapabilitySidebarItemView } from "./capability-sidebar-item";
import {
  buildCapabilitySidebarItems,
  type CapabilitySidebarItem,
} from "./capability-sidebar-model";

export const CapabilitySidebarPanel = memo(function CapabilitySidebarPanel() {
  const { scrollRef, fadeProps } = useScrollFade();
  const { t } = useI18n();
  const navigate = useNavigate();
  const activeItemId = useSidebarStore((state) => state.active_panel_item_id);
  const setActiveItem = useSidebarStore((state) => state.set_active_panel_item);
  const items = useMemo(() => buildCapabilitySidebarItems(t), [t]);
  const selectItem = useCallback((item: CapabilitySidebarItem) => {
    setActiveItem(item.id);
    navigate(item.path);
  }, [navigate, setActiveItem]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div
        ref={scrollRef}
        {...fadeProps}
        className="soft-scrollbar flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto pb-2 pl-2 pr-0.5 overscroll-contain max-[559px]:gap-1 max-[559px]:px-3"
      >
        {items.map((item) => (
          <CapabilitySidebarItemView
            active={activeItemId === item.id}
            item={item}
            key={item.id}
            onSelect={selectItem}
          />
        ))}
      </div>
    </div>
  );
});
