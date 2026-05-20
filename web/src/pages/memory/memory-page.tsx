"use client";

import { MemoryPanel } from "@/features/memory/memory-panel";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

export function MemoryPage() {
  return (
    <WorkspacePageFrame content_padding_class_name="p-0">
      <MemoryPanel />
    </WorkspacePageFrame>
  );
}
