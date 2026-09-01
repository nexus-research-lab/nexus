import { cn } from "@/shared/ui/class-name";

export type UiStateBlockSize = "sm" | "md" | "lg";
export type UiStateBlockTone = "default" | "danger" | "warning";
export type UiStateBlockVariant = "inset" | "card" | "plain";

interface UiStateBlockStyleOptions {
  size?: UiStateBlockSize;
  tone?: UiStateBlockTone;
  variant?: UiStateBlockVariant;
}

const STATE_BLOCK_BASE_CLASS_NAME =
  "flex flex-col items-center justify-center text-center";

const STATE_BLOCK_SIZE_CLASS_MAP: Record<UiStateBlockSize, string> = {
  sm: "min-h-32 surface-radius-sm px-4 py-5",
  md: "min-h-[240px] surface-radius-md px-5 py-6",
  lg: "min-h-[320px] surface-radius-lg px-6 py-8",
};

const STATE_BLOCK_VARIANT_CLASS_MAP: Record<UiStateBlockVariant, Record<UiStateBlockTone, string>> = {
  inset: {
    default: "border border-dashed border-(--divider-subtle-color) bg-transparent",
    danger: "border border-(--divider-subtle-color) bg-transparent",
    warning: "border border-(--divider-subtle-color) bg-transparent",
  },
  card: {
    default: "border border-(--divider-subtle-color) bg-transparent",
    danger: "border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--destructive)_2%,transparent)]",
    warning: "border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--warning)_2%,transparent)]",
  },
  plain: {
    default: "",
    danger: "",
    warning: "",
  },
};

export function getUiStateBlockClassName(
  options: UiStateBlockStyleOptions = {},
  className?: string,
): string {
  const {
    size = "md",
    tone = "default",
    variant = "inset",
  } = options;

  return cn(
    STATE_BLOCK_BASE_CLASS_NAME,
    STATE_BLOCK_SIZE_CLASS_MAP[size],
    STATE_BLOCK_VARIANT_CLASS_MAP[variant][tone],
    className,
  );
}
