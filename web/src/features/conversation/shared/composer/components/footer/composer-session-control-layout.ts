// INPUT: 当前 Room 菜单层级、可并排空间与 Agent/模型条目数量。
// OUTPUT: Session 模型菜单共用宽度、返回栏 recipe，以及当前可见内容的尺寸。
// POS: Composer 领域菜单内容几何；通用行高/间距和视口限制仍归 shared/menu、shared/overlay。

import { getMenuContentHeight, getMenuItemLayout } from "@/shared/ui/menu/menu-styles";
import {
  getUiAnchoredOverlayGap,
  getUiAnchoredOverlayMinimumWidth,
  getUiAnchoredOverlayViewportInset,
} from "@/shared/ui/overlay/anchored-overlay-layout";

export const SESSION_MODEL_MENU_WIDTH = 256;
export const ROOM_MODEL_AGENT_MENU_WIDTH = getUiAnchoredOverlayMinimumWidth("cascade-menu");
export const ROOM_MODEL_MENU_GAP = getUiAnchoredOverlayGap("cascade-menu");
export const ROOM_MODEL_CASCADE_QUERY = `(min-width: ${
  ROOM_MODEL_AGENT_MENU_WIDTH + SESSION_MODEL_MENU_WIDTH + ROOM_MODEL_MENU_GAP
  + getUiAnchoredOverlayViewportInset("cascade-menu") * 2
}px)`;

export const ROOM_MODEL_HEADER_LAYOUT = {
  className: "flex h-10 shrink-0 items-center gap-2 border-b border-(--divider-subtle-color) px-2",
  height: 40,
} as const;

export function getRoomModelMenuLayout({ agentCount, modelCount, modelsVisible, sideBySide }: {
  agentCount: number;
  modelCount: number;
  modelsVisible: boolean;
  sideBySide: boolean;
}) {
  const agentHeight = getMenuContentHeight(Array.from({ length: agentCount }, () => getMenuItemLayout().height));
  if (!modelsVisible) return { width: ROOM_MODEL_AGENT_MENU_WIDTH, height: agentHeight };
  const modelHeight = getMenuContentHeight(
    Array.from({ length: modelCount + 1 }, () => getMenuItemLayout({ density: "compact" }).height), 1,
  );
  return sideBySide ? {
    width: ROOM_MODEL_AGENT_MENU_WIDTH + SESSION_MODEL_MENU_WIDTH + ROOM_MODEL_MENU_GAP,
    height: Math.max(agentHeight, modelHeight),
  } : {
    width: SESSION_MODEL_MENU_WIDTH,
    height: ROOM_MODEL_HEADER_LAYOUT.height + modelHeight,
  };
}
