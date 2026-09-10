// INPUT: Dialog 的结构语义、视口模式和调用方外部布局约束。
// OUTPUT: Dialog 专属标题、图标与遮罩的稳定 className；行内说明复用公共 Notice。
// POS: Dialog 视觉几何入口；不处理焦点、modal 栈或业务提交。

export const DIALOG_HEADER_LEADING_CLASS_NAME = "flex min-w-0 items-center gap-2.5";

/** 统一弹窗遮罩 */
export const DIALOG_BACKDROP_CLASS_NAME =
  "dialog-backdrop animate-in fade-in duration-(--motion-duration-fast)";

export const DIALOG_HEADER_ICON_CLASS_NAME =
  "flex h-8 w-8 shrink-0 items-center justify-center radius-control-sm bg-(--surface-interactive-hover-background) text-(--icon-default)";
