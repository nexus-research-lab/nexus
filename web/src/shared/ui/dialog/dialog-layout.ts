// INPUT: Dialog 内容对工作区视口的占用语义。
// OUTPUT: content、紧凑/标准自适应、仅限高与大型工作台共享几何 recipe。
// POS: Dialog 响应式几何模型；不负责宽度、层级、焦点或滚动容器。

export type UiDialogViewport =
  | "content"
  | "compact"
  | "compactMax"
  | "adaptive"
  | "adaptiveMax"
  | "workbench";

const DIALOG_VIEWPORT_CLASS_MAP: Record<UiDialogViewport, string> = {
  content: "",
  compact: "ui-dialog-viewport-compact",
  compactMax: "ui-dialog-viewport-compact-max",
  adaptive: "ui-dialog-viewport-adaptive",
  adaptiveMax: "ui-dialog-viewport-adaptive-max",
  workbench: "ui-dialog-viewport-workbench",
};

export function getUiDialogViewportClassName(viewport: UiDialogViewport): string {
  return DIALOG_VIEWPORT_CLASS_MAP[viewport];
}
