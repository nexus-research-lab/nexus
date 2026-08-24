/**
 * INPUT: 拖动指针纵坐标与固定会话目标项几何。
 * OUTPUT: 目标项前/后的稳定插入位置。
 * POS: 固定会话拖放视图的纯几何模型。
 */
import type { SidebarPinnedConversationPlacement } from "./sidebar-wide-panel-types";

export function resolveSidebarPinnedConversationDropPlacement(
  clientY: number,
  itemTop: number,
  itemHeight: number,
): SidebarPinnedConversationPlacement {
  return clientY < itemTop + itemHeight / 2 ? "before" : "after";
}
