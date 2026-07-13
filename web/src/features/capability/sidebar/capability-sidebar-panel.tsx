/** 能力导航只组合摘要、搜索和路由，数据刷新与行呈现归各自职责模块。 */

import { memo, useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSearchInput } from "@/shared/ui/form/form-control";
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
      <div className="pb-2">
        <UiSearchInput
          className="w-full"
          inputClassName="text-[13px]"
          onChange={setQuery}
          placeholder={t("sidebar.search_capabilities")}
          value={query}
        />
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-1">
        {items.length > 0 ? items.map((item) => (
          <CapabilitySidebarItemView
            active={activeItemId === item.id}
            item={item}
            key={item.id}
            onSelect={selectItem}
          />
        )) : (
          <div className="px-2.5 py-4 text-[12px] text-(--text-muted)">
            {t("sidebar.no_matching_capabilities")}
          </div>
        )}
      </div>
    </div>
  );
});
