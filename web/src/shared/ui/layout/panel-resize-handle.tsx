// INPUT: 右侧面板的具名关联、有效像素宽度范围与调用方调整命令。
// OUTPUT: 可聚焦竖向分隔条，主键拖动、左右/Home/End 键调整及当前宽度语义。
// POS: 分栏交互 owner；有效宽度投影、尺寸状态和拖动生命周期归布局控制器。

"use client";

import type { MouseEventHandler } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

export interface PanelResizeControl {
  min: number;
  max: number;
  value: number;
  onChange: (width: number) => void;
}

interface PanelResizeHandleProps {
  ariaLabel: string;
  controls: string;
  control: PanelResizeControl | null;
  onResizeStart: MouseEventHandler<HTMLDivElement>;
  variant?: "gutter" | "overlay";
}

/** 右侧面板：左移分隔条扩大面板，右移缩小；不把调整动作解释为关闭页面。 */
export function PanelResizeHandle({
  ariaLabel,
  controls,
  control,
  onResizeStart,
  variant = "overlay",
}: PanelResizeHandleProps) {
  const { t } = useI18n();
  const disabled = !control || control.max <= control.min;
  return (
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- A focusable separator is a widget: https://www.w3.org/TR/wai-aria-1.2/#separator
    <div
      aria-controls={controls}
      aria-disabled={disabled || undefined}
      aria-label={ariaLabel}
      aria-orientation="vertical"
      aria-valuemin={control?.min ?? 0}
      aria-valuemax={control?.max ?? 0}
      aria-valuenow={control?.value ?? 0}
      aria-valuetext={control ? t("common.panel_width_pixels", { width: Math.round(control.value) }) : undefined}
      className={cn(
        "z-20 hidden h-full border-0 bg-transparent p-0 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:var(--ring)] lg:block",
        disabled ? "cursor-default" : "cursor-col-resize",
        variant === "gutter"
          ? "relative w-2 shrink-0 self-stretch"
          : "absolute left-0 top-0 w-3",
      )}
      onKeyDown={(event) => {
        if (disabled || !control || event.altKey || event.ctrlKey || event.metaKey) return;
        const next = event.key === "ArrowLeft" ? control.value + 16
          : event.key === "ArrowRight" ? control.value - 16
          : event.key === "Home" ? control.min
          : event.key === "End" ? control.max
          : null;
        if (next === null) return;
        event.preventDefault();
        event.stopPropagation();
        const width = Math.min(control.max, Math.max(control.min, next));
        if (width !== control.value) control.onChange(width);
      }}
      onMouseDown={(event) => {
        if (disabled || event.button !== 0) return;
        event.preventDefault();
        event.currentTarget.focus();
        onResizeStart(event);
      }}
      role="separator"
      tabIndex={disabled ? -1 : 0}
    />
  );
}
