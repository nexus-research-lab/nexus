// INPUT: 可展开标题、可选前导/元数据、正文、有限 surface/density 语义与原生 details 属性。
// OUTPUT: 统一焦点、展开箭头、排版和正文边界的原生 Disclosure。
// POS: Disclosure DOM 与视觉状态原语；不判断业务数据、权限或展开时机。

"use client";

import { ChevronDown } from "lucide-react";
import {
  type DetailsHTMLAttributes,
  type ReactNode,
  useState,
} from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiTypographyClassName,
  type UiTypographyRole,
  type UiTypographyTone,
  type UiTypographyWeight,
} from "@/shared/ui/typography/typography-styles";

export type UiDisclosureDensity = "compact" | "default";
export type UiDisclosureInset = "none" | "sm";
export type UiDisclosureVariant = "inline" | "panel" | "row" | "section";

interface UiDisclosureProps extends Omit<DetailsHTMLAttributes<HTMLDetailsElement>, "children" | "open"> {
  children: ReactNode;
  contentClassName?: string;
  defaultOpen?: boolean;
  density?: UiDisclosureDensity;
  inset?: UiDisclosureInset;
  label: ReactNode;
  leading?: ReactNode;
  meta?: ReactNode;
  open?: boolean;
  summaryRole?: Extract<UiTypographyRole, "caption" | "control" | "metadata" | "supporting">;
  summaryTone?: UiTypographyTone;
  summaryWeight?: UiTypographyWeight;
  surfaceTone?: "plain" | "subtle";
  variant?: UiDisclosureVariant;
}

const ROOT_CLASS_NAMES: Record<UiDisclosureVariant, string> = {
  inline: "group/disclosure",
  panel: "group/disclosure surface-radius-md border border-(--divider-subtle-color)",
  row: "group/disclosure",
  section: "group/disclosure border-t border-(--divider-subtle-color)",
};

const SUMMARY_CLASS_NAMES: Record<UiDisclosureVariant, Record<UiDisclosureDensity, string>> = {
  inline: {
    compact: "min-h-6 py-0.5",
    default: "min-h-7 py-1",
  },
  panel: {
    compact: "min-h-8 px-2 py-1.5",
    default: "min-h-10 px-3 py-2.5",
  },
  row: {
    compact: "min-h-9 radius-control-md px-2 py-1.5 hover:bg-(--surface-interactive-hover-background)",
    default: "min-h-10 radius-control-md px-2 py-2.5 hover:bg-(--surface-interactive-hover-background)",
  },
  section: {
    compact: "min-h-8 py-1.5",
    default: "min-h-9 py-2",
  },
};

const CONTENT_CLASS_NAMES: Record<UiDisclosureVariant, Record<UiDisclosureDensity, string>> = {
  inline: {
    compact: "mt-1.5",
    default: "mt-2",
  },
  panel: {
    compact: "border-t border-(--divider-subtle-color) px-2 py-2",
    default: "border-t border-(--divider-subtle-color) px-3 py-3",
  },
  row: {
    compact: "mx-2 border-l border-(--divider-subtle-color) pb-2.5 pl-3 pr-2",
    default: "mx-2 border-l border-(--divider-subtle-color) pb-3 pl-4 pr-2",
  },
  section: {
    compact: "pb-2.5 pt-1.5",
    default: "pb-3 pt-2",
  },
};

export function UiDisclosure({
  children,
  className,
  contentClassName,
  defaultOpen = false,
  density = "default",
  inset = "none",
  label,
  leading,
  meta,
  onToggle,
  open,
  summaryRole = "supporting",
  summaryTone = "default",
  summaryWeight = "medium",
  surfaceTone = "plain",
  variant = "inline",
  ...props
}: UiDisclosureProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(defaultOpen);
  const resolvedOpen = open ?? uncontrolledOpen;
  return (
    <details
      className={cn(
        ROOT_CLASS_NAMES[variant],
        variant === "panel" && surfaceTone === "subtle"
          && "bg-[color:color-mix(in_srgb,var(--surface-control-background)_64%,transparent)]",
        className,
      )}
      onToggle={(event) => {
        if (open === undefined) {
          setUncontrolledOpen(event.currentTarget.open);
        }
        onToggle?.(event);
      }}
      open={resolvedOpen}
      {...props}
    >
      <summary
        className={cn(
          "flex cursor-pointer list-none select-none items-center gap-2 outline-none transition-[background,color,box-shadow] duration-(--motion-duration-fast) hover:text-(--text-strong) focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] [&::-webkit-details-marker]:hidden",
          SUMMARY_CLASS_NAMES[variant][density],
          variant === "section" && inset === "sm" && "px-3",
          getUiTypographyClassName({
            role: summaryRole,
            tone: summaryTone,
            weight: summaryWeight,
          }),
        )}
      >
        {leading}
        <span className="min-w-0 flex-1">{label}</span>
        {meta ? <span className="min-w-0 shrink truncate">{meta}</span> : null}
        <ChevronDown
          aria-hidden="true"
          className="h-3.5 w-3.5 shrink-0 text-(--icon-muted) transition-transform duration-(--motion-duration-fast) group-open/disclosure:rotate-180"
        />
      </summary>
      <div className={cn(
        CONTENT_CLASS_NAMES[variant][density],
        variant === "section" && inset === "sm" && "px-3",
        contentClassName,
      )}>
        {children}
      </div>
    </details>
  );
}
