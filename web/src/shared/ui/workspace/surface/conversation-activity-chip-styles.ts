// INPUT: Composer 上方会话活动入口的布局 class 与平面摘要/控件表面语境。
// OUTPUT: 共用 metadata 排版；Task 使用轻量平面，Room 保留控件材质，多动作工具条使用 36px 外框。
// POS: Conversation activity chip 的视觉入口；业务组件只补充布局，不重定义字号。

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function getConversationActivityChipClassName(className?: string, surface: "control" | "plain" = "control"): string {
  return cn(
    "conversation-activity-chip",
    surface === "plain" && "conversation-activity-plain",
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
