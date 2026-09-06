// INPUT: Agent 身份页所在的弹窗或内联上下文。
// OUTPUT: 同一资料/标签/模型阅读顺序下的响应式布局。
// POS: 身份页只拥有布局差异；标签、文字、反馈与控件外观归共享 Field。

export type AgentIdentityVariant = "dialog" | "inline";

interface IdentityLayout {
  contentClassName: string;
  modelClassName: string;
  profileClassName: string;
  tagsClassName: string;
}

export const IDENTITY_LAYOUTS: Record<AgentIdentityVariant, IdentityLayout> = {
  dialog: {
    contentClassName: "grid grid-cols-1 gap-4",
    modelClassName: "min-w-0",
    profileClassName: "space-y-3",
    tagsClassName: "grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2",
  },
  inline: {
    contentClassName: "grid grid-cols-1 gap-5",
    modelClassName: "min-w-0",
    profileClassName: "min-w-0 space-y-4",
    tagsClassName: "grid min-w-0 grid-cols-1 gap-4 lg:grid-cols-2",
  },
};
