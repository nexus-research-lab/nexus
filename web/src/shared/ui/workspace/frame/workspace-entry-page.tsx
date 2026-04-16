"use client";

import { ReactNode } from "react";

import { WorkspaceEmptyState } from "./workspace-empty-state";
import { WorkspacePageFrame } from "./workspace-page-frame";

interface WorkspaceEntryPageProps {
  icon: ReactNode;
  title: string;
  description: string;
  actions?: ReactNode;
}

/** 通用空状态入口页 — 外层布局由路由层提供 */
export function WorkspaceEntryPage({
  icon,
  title,
  description,
  actions,
}: WorkspaceEntryPageProps) {
  return (
    <WorkspacePageFrame>
      <WorkspaceEmptyState
        actions={actions}
        description={description}
        icon={icon}
        title={title}
      />
    </WorkspacePageFrame>
  );
}
