// INPUT: 共享浮层和所属模态使用的打开状态语义。
// OUTPUT: 同一 DOM 打开标记与查询选择器。
// POS: Overlay 可观察状态合同；不替代关闭 runtime 的节点身份仲裁。
export const OPEN_OVERLAY_DATA_ATTRIBUTES = {
  "data-ui-overlay-open": "true",
} as const;

export const OPEN_OVERLAY_SELECTOR = "[data-ui-overlay-open='true']";
