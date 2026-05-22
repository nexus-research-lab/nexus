"use client";

import { SlidersHorizontal } from "lucide-react";

import { UiSearchInput } from "@/shared/ui/form-control";
import { UiSelectMenu } from "@/shared/ui/select-menu";

import { CONNECTOR_CATEGORY_OPTIONS, get_connector_category_label } from "./connectors-categories";
import type { ConnectorDirectoryController } from "./connectors-view-model";

interface ConnectorsSearchBarProps {
  ctrl: ConnectorDirectoryController;
}

export function ConnectorsSearchBar({ ctrl }: ConnectorsSearchBarProps) {
  return (
    <div className="mb-5 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center">
      <UiSearchInput
        class_name="h-10 min-w-0 flex-1 rounded-[13px] border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,white)] px-3.5"
        input_class_name="text-[14px]"
        on_change={ctrl.set_search_query}
        placeholder="搜索连接器"
        value={ctrl.search_query}
      />
      <UiSelectMenu
        aria_label="筛选连接器分类"
        class_name="shrink-0 sm:w-[184px]"
        label="分类"
        leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
        on_change={ctrl.set_active_category}
        options={CONNECTOR_CATEGORY_OPTIONS.map((item) => ({
          label: item.label,
          value: item.key,
        }))}
        placeholder={get_connector_category_label("all")}
        value={ctrl.active_category}
      />
    </div>
  );
}
