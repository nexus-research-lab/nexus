// INPUT: Composer 上方会话活动入口的外部布局 class。
// OUTPUT: 共用活动材质和 metadata 排版；Task/Room 保留紧凑基线，多动作工具条使用 36px 外框。
// POS: Conversation activity chip 的视觉入口；业务组件只补充布局，不重定义字号。

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function getConversationActivityChipClassName(className?: string): string {
  return cn(
    "conversation-activity-chip",
    getUiTypographyClassName({ role: "metadata" }),
    className,
  );
}

/** 多动作活动 Dock 保留 32px 命中区；36px 外框含边框与垂直留白。 */
export function getConversationActivityToolbarClassName(className?: string): string {
  return getConversationActivityChipClassName(cn(
    "conversation-activity-toolbar h-9 gap-1 px-1 py-px",
    className,
  ));
}
