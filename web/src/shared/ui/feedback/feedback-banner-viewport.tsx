// INPUT: 当前唯一的全局反馈条，或空状态。
// OUTPUT: 在桌面和窄屏中都不裁切正文的固定反馈视口。
// POS: 只管理视口位置，不持有反馈队列或业务状态。
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
    <div className="pointer-events-none fixed left-3 right-3 top-20 z-40 sm:left-auto sm:right-6 sm:top-24 sm:w-[460px] sm:max-w-[calc(100vw-3rem)]">
      <FeedbackBanner {...item} />
    </div>
  );
}
