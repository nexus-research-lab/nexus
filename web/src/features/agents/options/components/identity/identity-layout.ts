export type AgentIdentityVariant = "dialog" | "inline";

interface IdentityLayout {
  contentClassName: string;
  profileClassName: string;
  secondaryClassName: string;
}

export const IDENTITY_LAYOUTS: Record<AgentIdentityVariant, IdentityLayout> = {
  dialog: {
    contentClassName:
      "grid grid-cols-[minmax(0,1.1fr)_minmax(260px,0.9fr)] gap-5",
    profileClassName: "space-y-3",
    secondaryClassName: "space-y-4",
  },
  inline: {
    contentClassName:
      "flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between",
    profileClassName: "min-w-0 flex-1 space-y-3 xl:max-w-[480px]",
    secondaryClassName: "w-full space-y-4 pt-0.5 xl:w-[340px] xl:shrink-0",
  },
};
