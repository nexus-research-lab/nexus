// INPUT: 当前唯一的全局反馈条，或空状态。
// OUTPUT: 贴近页面右上角、保留安全区边距且不裁切正文的固定反馈视口。
// POS: 只管理视口位置，不持有反馈队列或业务状态。
import { cn } from "@/shared/ui/class-name";
import { getUiOverlayLayerClassName } from "@/shared/ui/overlay/layer-styles";

import {
  FeedbackBanner,
} from "./feedback-banner";
import type { FeedbackBannerProps } from "./feedback-banner-contract";

interface FeedbackBannerViewportProps {
  item: FeedbackBannerProps | null;
}

export function FeedbackBannerViewport({
  item,
}: FeedbackBannerViewportProps) {
  if (!item) {
    return null;
  }
  return (
    <div
      className={cn(
        "pointer-events-none fixed left-4 right-[max(1rem,env(safe-area-inset-right))] top-[max(1rem,env(safe-area-inset-top))] sm:left-auto sm:w-[460px] sm:max-w-[calc(100vw-2rem)]",
        getUiOverlayLayerClassName("feedback"),
      )}
      data-feedback-viewport
    >
      <FeedbackBanner {...item} />
    </div>
  );
}
