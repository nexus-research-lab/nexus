/**
 * =====================================================
 * @File   : feedback-banner-stack.tsx
 * @Date   : 2026-04-16 14:00
 * @Author : leemysw
 * 2026-04-16 14:00   Create
 * =====================================================
 */

import { FeedbackBanner } from "./feedback-banner";

export interface FeedbackBannerItem {
  key: string;
  tone: "success" | "warning" | "error";
  title: string;
  message: string;
  on_dismiss?: () => void;
}

interface FeedbackBannerStackProps {
  items: FeedbackBannerItem[];
}

export function FeedbackBannerStack({ items }: FeedbackBannerStackProps) {
  if (items.length === 0) {
    return null;
  }

  return (
    <div className="pointer-events-none fixed right-6 top-24 z-40 flex flex-col gap-2">
      {items.map((item) => (
        <FeedbackBanner
          key={item.key}
          message={item.message}
          on_dismiss={item.on_dismiss}
          title={item.title}
          tone={item.tone}
        />
      ))}
    </div>
  );
}
