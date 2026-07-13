/**
 * 工作台（/app）
 */

import { HomeAsciiHero } from "@/features/home/hero/home-ascii-hero";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";

export function HomePage() {
  return (
    <WorkspacePageFrame>
      <div className="flex min-h-0 flex-1 h-full">
        <HomeAsciiHero />
      </div>
    </WorkspacePageFrame>
  );
}
