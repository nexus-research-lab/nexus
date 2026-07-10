import { Loader2 } from "lucide-react";

import { getUiButtonClassName } from "@/shared/ui/button-styles";

export const CONTROL_CLASS_NAME =
  "dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)";
export const SAVE_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "sm", tone: "primary", variant: "solid" },
  "gap-1.5",
);
export const SECONDARY_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "sm", variant: "surface" },
  "gap-1.5",
);

export function SubscriptionLoadingState({ label }: { label: string }) {
  return (
    <div className="flex items-center justify-center gap-2 px-4 py-10 text-[12px] text-(--text-soft)">
      <Loader2 className="h-4 w-4 animate-spin" />
      {label}
    </div>
  );
}

export function SubscriptionEmptyState({ label }: { label: string }) {
  return (
    <div className="px-4 py-10 text-center text-[12px] text-(--text-soft)">
      {label}
    </div>
  );
}
