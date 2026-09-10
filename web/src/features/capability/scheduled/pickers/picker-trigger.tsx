// INPUT: Picker 的当前显示值、图标、展开状态、锚点引用与切换命令。
// OUTPUT: 复用共享 Button 的可访问日期/时间选择器触发器。
// POS: Scheduled Picker 的领域级触发器；不管理浮层生命周期或日期状态。

"use client";

import { type RefObject } from "react";

import { ChevronDown, type LucideIcon } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";

interface PickerTriggerProps {
  anchorRef: RefObject<HTMLButtonElement | null>;
  display: string;
  icon: LucideIcon;
  isOpen: boolean;
  label: string;
  onToggle: () => void;
}

export function PickerTrigger({
  anchorRef,
  display,
  icon: Icon,
  isOpen,
  label,
  onToggle,
}: PickerTriggerProps) {
  return (
    <UiButton
      aria-expanded={isOpen}
      aria-haspopup="dialog"
      aria-label={`${label}: ${display}`}
      className="w-full justify-between text-left"
      onClick={onToggle}
      ref={anchorRef}
      size="lg"
      variant="surface"
    >
      <span className="inline-flex min-w-0 items-center gap-2.5">
        <Icon aria-hidden="true" className="h-4 w-4 shrink-0 text-(--icon-muted)" />
        <span className="truncate">{display}</span>
      </span>
      <ChevronDown
        aria-hidden="true"
        className={cn(
          "h-4 w-4 shrink-0 text-(--icon-muted) transition-transform duration-(--motion-duration-fast)",
          isOpen && "rotate-180",
        )}
      />
    </UiButton>
  );
}
