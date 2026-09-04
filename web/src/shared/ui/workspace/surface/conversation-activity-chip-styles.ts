// INPUT: Composer 上方会话活动入口的外部布局 class。
// OUTPUT: Task、Room 协作与 WorkGraph 共用的紧凑材质和 metadata 排版配方。
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

/** 多动作活动 Dock 在 32px 控件外保留稳定内边距，不把按钮贴到 Surface 边缘。 */
export function getConversationActivityToolbarClassName(className?: string): string {
  return getConversationActivityChipClassName(cn(
    "min-h-10 gap-1 px-1 py-1",
    className,
  ));
}
