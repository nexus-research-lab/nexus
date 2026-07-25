/**
 * 工作台（/app）
 */

import { HomeAsciiHero } from "@/features/home/hero/home-ascii-hero";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

export function HomePage() {
  return (
    <>
      <header
        aria-hidden="true"
        className={`home-desktop-drag-header shell-region-header shrink-0 ${WORKSPACE_HEADER_HEIGHT_CLASS}`}
        data-desktop-window-drag-region
      />
      <WorkspacePageFrame>
        <div className="flex h-full min-h-0 flex-1">
          <HomeAsciiHero />
        </div>
      </WorkspacePageFrame>
    </>
  );
}
