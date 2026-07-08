"use client";

import { MemoryPanel } from "@/features/memory/memory-panel";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

export function MemoryPage() {
  return (
    <WorkspacePageFrame contentPaddingClassName="p-0">
      <MemoryPanel />
    </WorkspacePageFrame>
  );
}
