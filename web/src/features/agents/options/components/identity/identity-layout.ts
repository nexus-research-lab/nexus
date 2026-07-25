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
  secondaryClassName: string;
}

export const IDENTITY_LAYOUTS: Record<AgentIdentityVariant, IdentityLayout> = {
  dialog: {
    contentClassName:
      "grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1.08fr)_minmax(250px,0.92fr)] md:gap-6",
    modelClassName: "min-w-0 md:col-span-2",
    profileClassName: "space-y-3",
    secondaryClassName: "min-w-0",
  },
  inline: {
    contentClassName:
      "grid grid-cols-1 gap-x-8 gap-y-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(300px,0.85fr)]",
    modelClassName: "min-w-0 xl:col-span-2",
    profileClassName: "min-w-0 space-y-4",
    secondaryClassName: "min-w-0 pt-0.5",
  },
};
