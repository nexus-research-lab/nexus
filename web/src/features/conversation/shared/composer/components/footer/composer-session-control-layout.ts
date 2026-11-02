// INPUT: 当前菜单层级（Agent/模型/思考强度）、可并排空间与各层条目数量。
// OUTPUT: Session 模型菜单与强度侧栏的宽度、并排媒体查询、返回栏与分组标题 recipe，以及当前可见内容的尺寸。
// POS: Composer 领域菜单内容几何；通用行高/间距和视口限制仍归 shared/menu、shared/overlay。

import { getMenuContentHeight, getMenuItemLayout } from "@/shared/ui/menu/menu-styles";
import {
  getUiAnchoredOverlayGap,
  getUiAnchoredOverlayMinimumWidth,
  getUiAnchoredOverlayViewportInset,
} from "@/shared/ui/overlay/anchored-overlay-layout";

export const SESSION_MODEL_MENU_WIDTH = 256;
// DM 直接浮层：Provider 已移到分组标题，模型行只剩名称/默认标记，
// 288px 足够容纳强度入口的标签与当前值；Room 双栏仍沿用 256px。
export const SESSION_DIRECT_MODEL_MENU_WIDTH = 288;
// 思考强度侧栏：说明文字两行内放下，档位行留出警示徽标。
export const SESSION_EFFORT_MENU_WIDTH = 272;
export const ROOM_MODEL_AGENT_MENU_WIDTH = getUiAnchoredOverlayMinimumWidth("cascade-menu");
export const ROOM_MODEL_MENU_GAP = getUiAnchoredOverlayGap("cascade-menu");
export const ROOM_MODEL_CASCADE_QUERY = `(min-width: ${
  ROOM_MODEL_AGENT_MENU_WIDTH + SESSION_MODEL_MENU_WIDTH + ROOM_MODEL_MENU_GAP
  + getUiAnchoredOverlayViewportInset("cascade-menu") * 2
}px)`;

// DM 浮层能否把强度面板并排放在模型面板左侧；放不下时在模型面板内原位进入。
export const SESSION_EFFORT_CASCADE_QUERY = `(min-width: ${
  SESSION_DIRECT_MODEL_MENU_WIDTH + SESSION_EFFORT_MENU_WIDTH + ROOM_MODEL_MENU_GAP
  + getUiAnchoredOverlayViewportInset("cascade-menu") * 2
}px)`;

export const SESSION_MODEL_HEADER_LAYOUT = {
  className: "flex h-10 shrink-0 items-center gap-2 border-b border-(--divider-subtle-color) px-2",
  height: 40,
} as const;

/** 多 Provider 时的分组标题；不可交互，只参与高度估算和阅读分区。 */
export const SESSION_MODEL_GROUP_LABEL_LAYOUT = {
  className: "flex h-5 shrink-0 items-end px-2 pb-0.5",
  height: 20,
} as const;

// 思考强度说明是两行 metadata 加上下留白，估算按共享菜单 header 的实际几何。
const SESSION_EFFORT_HINT_HEIGHT = 48;

export interface SessionModelMenuShape {
  /** 每组模型数；多于一组时每组前多一个分组标题。 */
  groupSizes: number[];
  /** 底部动作行数：思考强度入口（可选）与重置。 */
  footerCount: number;
  effortCount: number;
}

/** 强度面板正文高度；原位子页额外计入返回栏，侧栏不带返回栏。 */
export function getSessionEffortMenuContentHeight(
  shape: SessionModelMenuShape,
  withHeader: boolean,
): number {
  const rowHeight = getMenuItemLayout({ density: "dense" }).height;
  return (withHeader ? SESSION_MODEL_HEADER_LAYOUT.height : 0) + getMenuContentHeight(
    [SESSION_EFFORT_HINT_HEIGHT, ...Array.from({ length: shape.effortCount }, () => rowHeight)], 1,
  );
}

/** 模型或强度视图的正文高度；强度视图额外计入返回栏。 */
export function getSessionModelMenuContentHeight(
  shape: SessionModelMenuShape,
  view: "models" | "effort",
): number {
  if (view === "effort") return getSessionEffortMenuContentHeight(shape, true);
  const rowHeight = getMenuItemLayout({ density: "dense" }).height;
  const labelled = shape.groupSizes.length > 1;
  const rows = shape.groupSizes.flatMap((size) => [
    ...(labelled ? [SESSION_MODEL_GROUP_LABEL_LAYOUT.height] : []),
    ...Array.from({ length: size }, () => rowHeight),
  ]);
  return getMenuContentHeight(
    [...rows, ...Array.from({ length: shape.footerCount }, () => rowHeight)], 1,
  );
}

export function getRoomModelMenuLayout({ agentCount, modelShape, modelView, modelsVisible, sideBySide }: {
  agentCount: number;
  modelShape: SessionModelMenuShape;
  modelView: "models" | "effort";
  modelsVisible: boolean;
  sideBySide: boolean;
}) {
  const agentHeight = getMenuContentHeight(Array.from({ length: agentCount }, () => getMenuItemLayout().height));
  if (!modelsVisible) return { width: ROOM_MODEL_AGENT_MENU_WIDTH, height: agentHeight };
  const modelHeight = getSessionModelMenuContentHeight(modelShape, modelView);
  return sideBySide ? {
    width: ROOM_MODEL_AGENT_MENU_WIDTH + SESSION_MODEL_MENU_WIDTH + ROOM_MODEL_MENU_GAP,
    height: Math.max(agentHeight, modelHeight),
  } : {
    width: SESSION_MODEL_MENU_WIDTH,
    // 强度视图自带返回栏，模型视图才需要 Agent 返回栏。
    height: (modelView === "effort" ? 0 : SESSION_MODEL_HEADER_LAYOUT.height) + modelHeight,
  };
}
