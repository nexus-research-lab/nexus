// INPUT: Humation item 层的坐标与颜色约定。
// OUTPUT: Nexus 自有徽章和终端配件。
// POS: 只追加素材，不修改上游部件及其稳定 ID。
import type { PartOption } from "./types";

export const nexusParts: PartOption[] = [
  {
    id: "nexus-badge", name: "Nexus badge", selectionSlot: "item", uiGroups: ["item"],
    layers: [{ layerSlot: "item", svg: '<svg xmlns="http://www.w3.org/2000/svg"><g stroke="var(--hm-stroke, #000000)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M58 57 Q68 54 75 59 L74 76 Q65 80 58 75 Z" fill="#B6DDF5"/><path d="M62 72 V62 L70 72 V62" fill="none"/></g></svg>' }],
  },
  {
    id: "nexus-terminal", name: "Nexus terminal", selectionSlot: "item", uiGroups: ["item"],
    layers: [{ layerSlot: "item", svg: '<svg xmlns="http://www.w3.org/2000/svg"><g stroke="var(--hm-stroke, #000000)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M48 55 Q62 53 77 55 L77 77 H48 Z" fill="#D8F0E5"/><path d="M54 62 L59 66 L54 70 M63 70 H70 M46 78 H79" fill="none"/></g></svg>' }],
  },
];
