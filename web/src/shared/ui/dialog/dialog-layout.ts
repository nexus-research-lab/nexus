// INPUT: Dialog 内容对工作区视口的占用语义。
// OUTPUT: content、固定自适应高度或仅限高三种共享几何 recipe。
// POS: Dialog 响应式几何模型；不负责宽度、层级、焦点或滚动容器。

export type UiDialogViewport = "content" | "adaptive" | "adaptiveMax";

const DIALOG_VIEWPORT_CLASS_MAP: Record<UiDialogViewport, string> = {
  content: "",
  adaptive: "ui-dialog-viewport-adaptive",
  adaptiveMax: "ui-dialog-viewport-adaptive-max",
};

export function getUiDialogViewportClassName(viewport: UiDialogViewport): string {
  return DIALOG_VIEWPORT_CLASS_MAP[viewport];
}
