// INPUT: 图标目录范围、当前选择、布局密度与选择命令。
// OUTPUT: 使用共享 Choice/Button 状态的网格或横向图标选择器。
// POS: IconPicker DOM 所有者；资源条目由纯模型生成，滚动测量由专用 Hook 持有。

"use client";

import type { CSSProperties } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";

import type { AvatarIconFamily } from "@/lib/avatar";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { useI18n } from "@/shared/i18n/i18n-context";

import { getIconPickerCollectionClassName } from "./icon-picker-layout";
import {
  buildIconPickerModel,
  type IconPickerColumns,
  type IconPickerLayout,
  type IconPickerSize,
} from "./icon-picker-model";
import { useIconPickerRowScroll } from "./use-icon-picker-row-scroll";

interface IconPickerProps {
  className?: string;
  columns?: IconPickerColumns;
  disabled?: boolean;
  iconFamily?: AvatarIconFamily;
  iconSize?: IconPickerSize;
  layout?: IconPickerLayout;
  maxIcons?: number;
  onSelect: (iconId: string) => void;
  showClear?: boolean;
  startIconId?: number;
  value?: string;
}

export function IconPicker({
  className,
  columns = 6,
  disabled = false,
  iconFamily = "agent",
  iconSize = "md",
  layout = "grid",
  maxIcons = 24,
  onSelect,
  showClear = true,
  startIconId = 1,
  value,
}: IconPickerProps) {
  const { t } = useI18n();
  const model = buildIconPickerModel({
    iconFamily,
    maxIcons,
    showClear,
    startIconId,
    value,
  });
  const rowScroll = useIconPickerRowScroll({
    enabled: layout === "row",
    itemCount: model.items.length,
  });
  const scrollProgress = rowScroll.metrics.maxScrollLeft > 0
    ? rowScroll.metrics.scrollLeft / rowScroll.metrics.maxScrollLeft
    : 0;
  const scrollRangeStyle = {
    "--icon-picker-scroll-progress": `${scrollProgress * 100}%`,
  } as CSSProperties;

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      {model.showClear ? (
        <UiButton
          disabled={disabled}
          onClick={() => onSelect("")}
          size="2xs"
          variant="text"
        >
          <X className="h-3.5 w-3.5" />
          {t("common.clear")}
        </UiButton>
      ) : null}
      <div
        ref={rowScroll.collectionRef}
        className={getIconPickerCollectionClassName({ columns, layout })}
      >
        {model.items.map((item) => (
          <UiChoiceButton
            active={item.active}
            aria-label={item.title}
            choiceSize={iconSize}
            className={layout === "row" ? "shrink-0" : undefined}
            disabled={disabled}
            key={item.iconId}
            onClick={() => onSelect(item.iconId)}
            title={item.title}
            variant="icon"
          >
            <img
              alt=""
              className="h-full w-full rounded-[inherit] object-cover"
              crossOrigin="anonymous"
              src={item.iconPath}
            />
          </UiChoiceButton>
        ))}
      </div>
      {layout === "row" && rowScroll.metrics.canScroll ? (
        <div className="flex items-center gap-2 px-0.5" data-icon-picker-scroll-controls="true">
          <UiIconButton
            aria-label={t("common.icon_picker_previous")}
            disabled={!rowScroll.metrics.canScrollBackward}
            onClick={rowScroll.scrollBackward}
            size="sm"
            variant="surface"
          >
            <ChevronLeft className="h-3.5 w-3.5" />
          </UiIconButton>
          <input
            aria-label={t("common.icon_picker_scroll")}
            className="icon-picker-scroll-range min-w-0 flex-1"
            max={rowScroll.metrics.maxScrollLeft}
            min={0}
            onChange={(event) => rowScroll.setScrollLeft(Number(event.target.value))}
            step={1}
            style={scrollRangeStyle}
            type="range"
            value={rowScroll.metrics.scrollLeft}
          />
          <UiIconButton
            aria-label={t("common.icon_picker_next")}
            disabled={!rowScroll.metrics.canScrollForward}
            onClick={rowScroll.scrollForward}
            size="sm"
            variant="surface"
          >
            <ChevronRight className="h-3.5 w-3.5" />
          </UiIconButton>
        </div>
      ) : null}
    </div>
  );
}
