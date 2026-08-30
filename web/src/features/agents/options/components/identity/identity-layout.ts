export type AgentIdentityVariant = "dialog" | "inline";

export const IDENTITY_FIELD_LABEL_CLASS_NAMES = {
  dialog: "text-xs font-semibold text-(--text-muted)",
  inline:
    "text-xs font-semibold uppercase tracking-[0.12em] text-(--text-soft)",
} as const satisfies Record<AgentIdentityVariant, string>;

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
