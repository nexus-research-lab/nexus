"use client";

import { Link2 } from "lucide-react";

import type { ConnectorController } from "@/hooks/use-connector-controller";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/workspace-search-input";

interface ConnectorsSearchBarProps {
  ctrl: ConnectorController;
}

export function ConnectorsSearchBar({ ctrl }: ConnectorsSearchBarProps) {
  return (
    <div className="mb-5 flex flex-wrap items-center gap-x-5 gap-y-3">
      <WorkspaceSearchInput
        class_name="h-11 w-full max-w-[34rem] px-3.5 py-2"
        input_class_name="text-[15px]"
        on_change={ctrl.set_search_query}
        placeholder="搜索应用授权..."
        value={ctrl.search_query}
      />
      <div className="inline-flex items-center gap-1.5 px-1 text-[12px] font-medium text-(--text-soft)">
        <Link2 className="h-3 w-3" />
        <span>{ctrl.connectors.length} 个应用授权</span>
      </div>
    </div>
  );
}
