export type AgentIdentityVariant = "dialog" | "inline";

interface IdentityLayout {
  contentClassName: string;
  profileClassName: string;
  secondaryClassName: string;
}

export const IDENTITY_LAYOUTS: Record<AgentIdentityVariant, IdentityLayout> = {
  dialog: {
    contentClassName:
      "grid grid-cols-1 gap-5 md:grid-cols-[minmax(0,1.1fr)_minmax(260px,0.9fr)] md:gap-7",
    profileClassName: "space-y-4",
    secondaryClassName: "space-y-5",
  },
  inline: {
    contentClassName:
      "flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between xl:gap-6",
    profileClassName: "min-w-0 space-y-4 xl:flex-1 xl:max-w-[500px]",
    secondaryClassName: "w-full space-y-5 pt-0.5 xl:w-[360px] xl:shrink-0",
  },
};
