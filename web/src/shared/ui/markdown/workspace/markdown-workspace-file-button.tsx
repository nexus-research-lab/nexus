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
      className="message-code-font inline-flex max-w-full items-center overflow-hidden rounded-[4px] border border-primary/20 bg-primary/10 px-1.5 py-0.5 text-left align-baseline text-[0.86em] leading-[1.25] text-primary transition-colors hover:border-primary/30 hover:bg-primary/15"
      onClick={() => onOpenWorkspaceFile(path, workspaceAgentId)}
      title={`Open ${path}`}
      type="button"
    >
      <span className="max-w-full whitespace-pre-wrap break-words">{label}</span>
    </button>
  );
}
