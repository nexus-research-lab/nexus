"use client";

import { ReactNode } from "react";

import { AppStage } from "@/shared/ui/app-stage";
import { AppGlobalRailKey } from "@/shared/ui/app-global-rail";

import { WorkspaceEmptyState } from "./workspace-empty-state";
import { WorkspacePageFrame } from "./workspace-page-frame";

interface WorkspaceEntryPageProps {
  active_rail_item: AppGlobalRailKey;
  icon: ReactNode;
  title: string;
  description: string;
  actions?: ReactNode;
}

export function WorkspaceEntryPage({
  active_rail_item,
  icon,
  title,
  description,
  actions,
}: WorkspaceEntryPageProps) {
  return (
    <AppStage active_rail_item={active_rail_item}>
      <WorkspacePageFrame>
        <WorkspaceEmptyState
          actions={actions}
          description={description}
          icon={icon}
          title={title}
        />
      </WorkspacePageFrame>
    </AppStage>
  );
}
