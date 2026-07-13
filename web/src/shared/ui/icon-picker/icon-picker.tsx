"use client";

import { X } from "lucide-react";

import type { AvatarIconFamily } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  getIconPickerPresentation,
  type IconPickerColumns,
  type IconPickerLayout,
  type IconPickerSize,
} from "./icon-picker-model";

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

  return (
    <div className={cn("flex flex-col gap-3", className)}>
      {presentation.showClear ? (
        <button
          className="inline-flex items-center gap-1.5 text-[12px] font-semibold text-(--text-muted) transition hover:text-(--text-default)"
          disabled={disabled}
          onClick={() => onSelect("")}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
          {t("common.clear")}
        </button>
      ) : null}
      <div className={presentation.collectionClassName}>
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
    </div>
  );
}
