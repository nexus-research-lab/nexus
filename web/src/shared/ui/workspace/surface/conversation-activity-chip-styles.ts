// INPUT: Composer 上方会话活动入口的布局 class 与控件表面语境。
// OUTPUT: Task、WorkGraph Dock 与 Room 协作状态共用的 metadata 排版、控件材质和活动间距。
// POS: Conversation activity chip 的视觉入口；业务组件只补充布局，不重定义字号。

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

/** 活动条与相邻工作栈节点之间的唯一紧凑间距。 */
export const CONVERSATION_ACTIVITY_STACK_GAP_CLASS_NAME = "mb-2";
export const CONVERSATION_ACTIVITY_STACK_GAP_PX = 8;
/** Goal 在 Composer 上方保留一小档上移，正文避让由尾部 spacer 完整承接。 */
export const CONVERSATION_ACTIVITY_STACK_CLEARANCE_CLASS_NAME = "pb-3";
export const CONVERSATION_ACTIVITY_STACK_MIN_CLEARANCE_PX = 56;
export const CONVERSATION_ACTIVITY_STACK_OFFSET_CLASS_NAME =
  "-translate-y-[calc(100%+0.5rem)]";

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
