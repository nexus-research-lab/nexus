// INPUT: 浮层的产品语义层级，而不是调用方挑选的整数。
// OUTPUT: 与主题 layer token 对应的稳定 className。
// POS: Web 高层浮层的层级入口；不负责 modal 顺序、定位或 Portal 生命周期。

export type UiOverlayLayer =
  | "selectMenu"
  | "actionMenu"
  | "popover"
  | "feedback"
  | "dialogUnderlay"
  | "dialog"
  | "dialogNested"
  | "dialogInteraction"
  | "tooltip"
  | "tour"
  | "tourDialog"
  | "systemDialog";

export type UiDialogLayer = Exclude<
  UiOverlayLayer,
  "selectMenu" | "actionMenu" | "popover" | "feedback" | "tooltip" | "tour"
>;

const UI_OVERLAY_LAYER_CLASS_MAP: Record<UiOverlayLayer, string> = {
  selectMenu: "ui-layer-select-menu",
  actionMenu: "ui-layer-action-menu",
  popover: "ui-layer-popover",
  feedback: "ui-layer-feedback",
  dialogUnderlay: "ui-layer-dialog-underlay",
  dialog: "ui-layer-dialog",
  dialogNested: "ui-layer-dialog-nested",
  dialogInteraction: "ui-layer-dialog-interaction",
  tooltip: "ui-layer-tooltip",
  tour: "ui-layer-tour",
  tourDialog: "ui-layer-tour-dialog",
  systemDialog: "ui-layer-system-dialog",
};

export function getUiOverlayLayerClassName(layer?: UiOverlayLayer): string | undefined {
  return layer ? UI_OVERLAY_LAYER_CLASS_MAP[layer] : undefined;
}
