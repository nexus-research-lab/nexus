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
      className="message-cjk-code-font inline-flex max-w-full items-center overflow-hidden rounded-[5px] border border-primary/20 bg-primary/10 px-2 py-0.4 text-left align-middle text-[13px] text-primary transition-colors hover:border-primary/30 hover:bg-primary/15"
      onClick={() => onOpenWorkspaceFile(path, workspaceAgentId)}
      title={`Open ${path}`}
      type="button"
    >
      <span className="max-w-full whitespace-pre-wrap break-words">{label}</span>
    </button>
  );
}
