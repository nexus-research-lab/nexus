import { cn } from "@/lib/utils";

export type UiStateBlockSize = "sm" | "md" | "lg";
export type UiStateBlockTone = "default" | "danger";
export type UiStateBlockVariant = "inset" | "card" | "plain";

interface UiStateBlockStyleOptions {
  size?: UiStateBlockSize;
  tone?: UiStateBlockTone;
  variant?: UiStateBlockVariant;
}

const STATE_BLOCK_BASE_CLASS_NAME =
  "flex flex-col items-center justify-center text-center";

const STATE_BLOCK_SIZE_CLASS_MAP: Record<UiStateBlockSize, string> = {
  sm: "min-h-32 rounded-[10px] px-4 py-5",
  md: "min-h-[240px] rounded-[12px] px-5 py-6",
  lg: "min-h-[320px] rounded-[14px] px-6 py-8",
};

const STATE_BLOCK_VARIANT_CLASS_MAP: Record<UiStateBlockVariant, Record<UiStateBlockTone, string>> = {
  inset: {
    default: "border border-dashed border-(--divider-subtle-color) bg-transparent",
    danger: "border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] bg-transparent",
  },
  card: {
    default: "border border-(--divider-subtle-color) bg-transparent",
    danger: "border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_4%,transparent)]",
  },
  plain: {
    default: "",
    danger: "",
  },
};

export function get_ui_state_block_class_name(
  options: UiStateBlockStyleOptions = {},
  class_name?: string,
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
    class_name,
  );
}
