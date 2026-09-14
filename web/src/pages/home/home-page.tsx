/**
 * INPUT: 当前桌面平台与共享工作区页框。
 * OUTPUT: 避开 macOS 标题栏的首页画布与原生拖动面。
 * POS: 工作台（/app）页面装配。
 */

import { HomeAsciiHero } from "@/features/home/hero/home-ascii-hero";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

export function HomePage() {
  return (
    <>
      <header
        aria-hidden="true"
        className={`home-desktop-drag-header pointer-events-auto absolute inset-x-0 top-0 z-10 ${WORKSPACE_HEADER_HEIGHT_CLASS}`}
        data-desktop-window-drag-region
      />
      <WorkspacePageFrame contentPaddingClassName="home-workspace-frame">
        <div className="flex h-full min-h-0 flex-1">
          <HomeAsciiHero />
        </div>
      </WorkspacePageFrame>
    </>
  );
}
