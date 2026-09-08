// INPUT: 主动作、可选菜单动作及共享禁用/尺寸约束。
// OUTPUT: 具有单一外框、独立焦点和原生 button 语义的紧凑 split action。
// POS: Button 组合 pattern；不管理菜单开关、业务权限或动作事务。
"use client";

import type {
  ButtonHTMLAttributes,
  HTMLAttributes,
  ReactNode,
  Ref,
} from "react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";

type UiSplitButtonAction = Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children" | "className" | "type"
> & {
  children: ReactNode;
  [dataAttribute: `data-${string}`]: string | number | boolean | undefined;
};

interface UiSplitButtonProps extends Omit<
  HTMLAttributes<HTMLDivElement>,
  "aria-label" | "children" | "className" | "role"
> {
  ariaLabel: string;
  className?: string;
  mainAction: UiSplitButtonAction;
  menuAction?: UiSplitButtonAction & { "aria-label": string };
  menuButtonRef?: Ref<HTMLButtonElement>;
}

/** 主动作与相邻菜单动作共享一个边界，但保留两个独立可访问命令。 */
export function UiSplitButton({
  ariaLabel,
  className,
  mainAction,
  menuAction,
  menuButtonRef,
  ...props
}: UiSplitButtonProps) {
  return (
    <div
      {...props}
      aria-label={ariaLabel}
      className={cn(
        "radius-control-sm flex min-h-8 items-stretch overflow-hidden",
        className,
      )}
      data-slot="split-button"
      role="group"
    >
      <UiButton
        {...mainAction}
        className="h-full min-h-0 min-w-0 flex-1 !rounded-none border-r-0 px-1.5 focus-visible:ring-inset"
        size="sm"
        tone="primary"
        variant="solid"
      />
      {menuAction ? (
        <UiButton
          ref={menuButtonRef}
          {...menuAction}
          className="h-full min-h-0 w-8 shrink-0 !rounded-none px-0 focus-visible:ring-inset"
          size="sm"
          tone="primary"
          variant="solid"
        />
      ) : null}
    </div>
  );
}
