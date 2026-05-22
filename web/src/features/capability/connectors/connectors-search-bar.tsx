"use client";

import { Link2 } from "lucide-react";

import { UiBadge } from "@/shared/ui/badge";
import { UiSearchInput, UiSelect } from "@/shared/ui/form-control";

import { CONNECTOR_CATEGORY_OPTIONS, get_connector_category_label } from "./connectors-categories";
import type { ConnectorDirectoryController } from "./connectors-view-model";

interface ConnectorsSearchBarProps {
  ctrl: ConnectorDirectoryController;
}

export function ConnectorsSearchBar({ ctrl }: ConnectorsSearchBarProps) {
  const active_category_label = get_connector_category_label(ctrl.active_category);

  return (
    <div className="mb-5 flex flex-wrap items-center justify-center gap-2.5">
      <UiSearchInput
        class_name="h-9 w-full max-w-[520px] rounded-[12px] border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_92%,white)] px-3"
        input_class_name="text-[14px]"
        on_change={ctrl.set_search_query}
        placeholder="搜索连接器"
        value={ctrl.search_query}
      />
      <UiSelect
        aria-label="筛选连接器分类"
        class_name="w-[132px]"
        onChange={(event) => ctrl.set_active_category(event.target.value)}
        value={ctrl.active_category}
        variant="surface"
      >
        {CONNECTOR_CATEGORY_OPTIONS.map((item) => (
          <option key={item.key} value={item.key}>
            {item.label}
          </option>
        ))}
      </UiSelect>
      <UiBadge class_name="h-9 px-3" size="md">
        <Link2 className="h-3 w-3" />
        <span>{active_category_label} · {ctrl.connectors.length}</span>
      </UiBadge>
    </div>
  );
}
