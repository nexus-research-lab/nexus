// INPUT: 列表次动作的可见性语义。
// OUTPUT: 常显、弱显与行 hover/focus 展示规则。
// POS: ListAction 可见性配方；颜色、尺寸、圆角与 disabled 状态归 IconButton。

export type UiListActionVisibility = "subtle" | "visible" | "hover";

const LIST_ACTION_VISIBILITY_CLASS_MAP: Record<UiListActionVisibility, string> = {
  subtle: "opacity-60 [&:not(:disabled):hover]:opacity-100 focus-visible:opacity-100",
  visible: "opacity-100",
  hover: "opacity-0 group-hover/item:[&:not(:disabled)]:opacity-100 group-focus-within/item:[&:not(:disabled)]:opacity-100 focus-visible:opacity-100 [@media(hover:none)]:opacity-100",
};

export function getUiListActionVisibilityClassName(visibility: UiListActionVisibility): string {
  return LIST_ACTION_VISIBILITY_CLASS_MAP[visibility];
}
