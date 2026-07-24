export type AgentIdentityVariant = "dialog" | "inline";

interface IdentityLayout {
  contentClassName: string;
  modelClassName: string;
  profileClassName: string;
  secondaryClassName: string;
}

export const IDENTITY_LAYOUTS: Record<AgentIdentityVariant, IdentityLayout> = {
  dialog: {
    contentClassName:
      "grid grid-cols-1 gap-5 md:grid-cols-[minmax(0,1.1fr)_minmax(260px,0.9fr)] md:gap-7",
    modelClassName: "",
    profileClassName: "space-y-4",
    secondaryClassName: "space-y-5",
  },
  inline: {
    contentClassName:
      "grid grid-cols-1 gap-x-8 gap-y-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(300px,0.85fr)]",
    modelClassName: "min-w-0 xl:col-span-2",
    profileClassName: "min-w-0 space-y-4",
    secondaryClassName: "min-w-0 pt-0.5",
  },
};
