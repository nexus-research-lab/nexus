"use client";

import { ReactNode } from "react";

import { AppStage } from "@/shared/ui/app-stage";

import { WorkspaceEmptyState } from "./workspace-empty-state";
import { WorkspacePageFrame } from "./workspace-page-frame";

interface WorkspaceEntryPageProps {
  icon: ReactNode;
  title: string;
  description: string;
  actions?: ReactNode;
}

export function WorkspaceEntryPage({
  icon,
  title,
  description,
  actions,
}: WorkspaceEntryPageProps) {
  return (
    <AppStage>
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
