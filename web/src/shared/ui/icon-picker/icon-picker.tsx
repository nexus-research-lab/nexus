"use client";

import type { CSSProperties } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";

import type { AvatarIconFamily } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { UiIconButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  getIconPickerPresentation,
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
  const presentation = getIconPickerPresentation({
    columns,
    disabled,
    iconFamily,
    iconSize,
    layout,
    maxIcons,
    showClear,
    startIconId,
    value,
  });
  const rowScroll = useIconPickerRowScroll({
    enabled: layout === "row",
    itemCount: presentation.items.length,
  });
  const scrollProgress = rowScroll.metrics.maxScrollLeft > 0
    ? rowScroll.metrics.scrollLeft / rowScroll.metrics.maxScrollLeft
    : 0;
  const scrollRangeStyle = {
    "--icon-picker-scroll-progress": `${scrollProgress * 100}%`,
  } as CSSProperties;

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      {presentation.showClear ? (
        <button
          className="inline-flex items-center gap-1.5 text-compact font-semibold text-(--text-muted) transition hover:text-(--text-default)"
          disabled={disabled}
          onClick={() => onSelect("")}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
          {t("common.clear")}
        </button>
      ) : null}
      <div
        ref={rowScroll.collectionRef}
        className={presentation.collectionClassName}
      >
        {presentation.items.map((item) => (
          <button
            className={item.className}
            disabled={disabled}
            key={item.iconId}
            onClick={() => onSelect(item.iconId)}
            title={item.title}
            type="button"
          >
            <img
              alt={item.title}
              className="h-full w-full rounded-[inherit] object-cover"
              crossOrigin="anonymous"
              src={item.iconPath}
            />
          </button>
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
