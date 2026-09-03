// INPUT: 能力摘要、当前选中项、搜索查询与路由导航命令。
// OUTPUT: 使用共享搜索、列表行和语义排版的能力目录侧栏。
// POS: 能力导航装配层；数据刷新和单行呈现归相邻 model/view。

import { memo, useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { SidebarSearchField } from "@/shared/ui/form/sidebar-search-field";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { useSidebarStore } from "@/store/sidebar";

import { CapabilitySidebarItemView } from "./capability-sidebar-item";
import {
  buildCapabilitySidebarItems,
  type CapabilitySidebarItem,
  filterCapabilitySidebarItems,
} from "./capability-sidebar-model";
import { useCapabilitySummary } from "./use-capability-summary";

export const CapabilitySidebarPanel = memo(function CapabilitySidebarPanel() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const activeItemId = useSidebarStore((state) => state.active_panel_item_id);
  const setActiveItem = useSidebarStore((state) => state.set_active_panel_item);
  const [query, setQuery] = useState("");
  const summary = useCapabilitySummary();
  const items = useMemo(
    () => filterCapabilitySidebarItems(
      buildCapabilitySidebarItems(summary, t),
      query,
    ),
    [query, summary, t],
  );
  const selectItem = useCallback((item: CapabilitySidebarItem) => {
    setActiveItem(item.id);
    navigate(item.path);
  }, [navigate, setActiveItem]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <SidebarSearchField
        onChange={setQuery}
        placeholder={t("sidebar.search_capabilities")}
        value={query}
      />

      <div className="flex min-h-0 flex-1 flex-col gap-0.5 px-2 pb-2 max-[559px]:gap-1 max-[559px]:px-3">
        {items.length > 0 ? items.map((item) => (
          <CapabilitySidebarItemView
            active={activeItemId === item.id}
            item={item}
            key={item.id}
            onSelect={selectItem}
          />
        )) : (
          <div className={cn(
            "px-2.5 py-4",
            getUiTypographyClassName({ role: "caption", tone: "muted" }),
          )}>
            {t("sidebar.no_matching_capabilities")}
          </div>
        )}
      </div>
    </div>
  );
});
