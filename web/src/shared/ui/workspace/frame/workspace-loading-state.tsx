"use client";

import { Loader2 } from "lucide-react";

interface WorkspaceLoadingStateProps {
  label: string;
}

export function WorkspaceLoadingState({ label }: WorkspaceLoadingStateProps) {
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center">
      <div className="flex flex-col items-center gap-3">
        <Loader2 className="h-6 w-6 animate-spin text-(--text-soft)" />
        <span className="text-sm text-(--text-soft)">{label}</span>
      </div>
    </div>
  );
}
