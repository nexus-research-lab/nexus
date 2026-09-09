// INPUT: Agent 身份页所在的弹窗或内联上下文。
// OUTPUT: 同一资料/标签/模型阅读顺序下按容器宽度排布的布局。
// POS: 身份页只拥有布局差异；标签、文字、反馈与控件外观归共享 Field。

export type AgentIdentityVariant = "dialog" | "inline";

export const IDENTITY_CONTENT_CLASS_NAMES: Record<AgentIdentityVariant, string> = {
  dialog: "grid min-w-0 grid-cols-1 gap-4",
  inline: "grid min-w-0 grid-cols-1 gap-5",
};

// 两个标签字段按表单可用宽度排布，最窄 15rem；不足时单列且不撑出容器。
export const IDENTITY_TAGS_CLASS_NAME =
  "grid min-w-0 grid-cols-[repeat(auto-fit,minmax(min(100%,15rem),1fr))] gap-4";
