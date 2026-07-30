"use client";

import { type ReactNode } from "react";

interface WorkspaceFileButtonProps {
  label: ReactNode;
  path: string;
  onOpenWorkspaceFile: (path: string, workspaceAgentId?: string | null) => void;
  workspaceAgentId?: string | null;
}

export function WorkspaceFileButton({
  label,
  path,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  workspaceAgentId: workspaceAgentId,
}: WorkspaceFileButtonProps) {
  return (
    <button
      className="content-workspace-file message-code-font max-w-full px-1.5 py-0.5 text-left align-baseline text-[0.86em] leading-[1.25]"
      onClick={() => onOpenWorkspaceFile(path, workspaceAgentId)}
      title={`Open ${path}`}
      type="button"
    >
      <span className="max-w-full whitespace-pre-wrap break-words">{label}</span>
    </button>
  );
}
