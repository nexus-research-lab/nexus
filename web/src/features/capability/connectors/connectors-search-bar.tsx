"use client";

import { Link2 } from "lucide-react";

import type { ConnectorController } from "@/hooks/use-connector-controller";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/workspace-search-input";

interface ConnectorsSearchBarProps {
  ctrl: ConnectorController;
}

export function ConnectorsSearchBar({ ctrl }: ConnectorsSearchBarProps) {
  return (
    <div className="mb-5 flex items-center gap-3">
      <WorkspaceSearchInput
        class_name="flex-1"
        on_change={ctrl.set_search_query}
        placeholder="搜索应用授权..."
        value={ctrl.search_query}
      />
      <div className="inline-flex items-center gap-1.5 px-1 text-xs font-medium text-(--text-soft)">
        <Link2 className="h-3.5 w-3.5" />
        <span>{ctrl.connectors.length} 个应用授权</span>
      </div>
    </div>
  );
}
