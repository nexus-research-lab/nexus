import {
  FeedbackBanner,
  type FeedbackBannerProps,
} from "./feedback-banner";

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
    <div className="pointer-events-none fixed right-6 top-24 z-40">
      <FeedbackBanner {...item} />
    </div>
  );
}
