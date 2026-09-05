// INPUT: Icon artwork, frame size, shape, semantic tone and native container attributes.
// OUTPUT: A shared decorative icon frame using surface and semantic color tokens.
// POS: Catalog icon presentation; interactive hit areas belong to the surrounding action owner.

import type { CSSProperties, HTMLAttributes, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

type IconFrameShape = "round" | "rounded";
type IconFrameTone = "default" | "primary" | "success" | "warning";
type IconFrameSize = "sm" | "md" | "lg";

const TONE_CLASSES: Record<IconFrameTone, string> = {
  default: "border-(--surface-control-border) bg-(--surface-control-background) text-(--text-default)",
  primary: "",
  success: "",
  warning: "",
};
const TONE_STYLES: Record<Exclude<IconFrameTone, "default">, CSSProperties> = {
  primary: {
    background: "color-mix(in srgb, var(--primary) 14%, var(--chip-default-background))",
    border: "1px solid color-mix(in srgb, var(--primary) 32%, var(--chip-default-border))",
    color: "color-mix(in srgb, var(--primary) 88%, var(--text-strong))",
  },
  success: {
    background: "color-mix(in srgb, var(--success) 16%, var(--chip-default-background))",
    border: "1px solid color-mix(in srgb, var(--success) 32%, var(--chip-default-border))",
    color: "color-mix(in srgb, var(--success) 84%, var(--text-strong))",
  },
  warning: {
    background: "color-mix(in srgb, var(--warning) 16%, var(--chip-default-background))",
    border: "1px solid color-mix(in srgb, var(--warning) 34%, var(--chip-default-border))",
    color: "color-mix(in srgb, var(--warning) 84%, var(--text-strong))",
  },
};
const SIZE_CLASSES: Record<IconFrameSize, string> = {
  sm: "h-9 w-9 radius-control-md",
  md: "h-11 w-11 radius-control-lg",
  lg: "h-14 w-14 surface-radius-md",
};

export function WorkspaceIconFrame({
  children,
  className,
  shape = "rounded",
  size = "md",
  tone = "default",
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  children: ReactNode;
  shape?: IconFrameShape;
  size?: IconFrameSize;
  tone?: IconFrameTone;
}) {
  return (
    <div
      className={cn(
        "flex shrink-0 items-center justify-center border shadow-(--surface-control-shadow)",
        SIZE_CLASSES[size],
        TONE_CLASSES[tone],
        shape === "round" && "rounded-full",
        className,
      )}
      style={tone === "default" ? undefined : TONE_STYLES[tone]}
      {...props}
    >
      {children}
    </div>
  );
}
