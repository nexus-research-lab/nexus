"use client";

/**
 * 通用图标选择器
 * 
 * 用于 Room Avatar、Agent Avatar 等场景。
 * 支持按目录族选择图标：icon/agent/{number}.png、icon/room/{number}.png
 */

import { X } from "lucide-react";
import { useMemo } from "react";

import type { AvatarIconFamily } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";

interface IconPickerProps {
    value?: string; // e.g. "13"
    onSelect: (iconId: string) => void;
    maxIcons?: number; // 默认 24
    startIconId?: number; // 默认 1
    columns?: number; // 默认 4
    layout?: "grid" | "row";
    iconSize?: "sm" | "md" | "lg"; // 默认 md
    iconFamily?: AvatarIconFamily; // 默认 agent
    showClear?: boolean;
    disabled?: boolean;
    className?: string;
}

const ICON_SIZE_MAP = {
    sm: "h-8 w-8",
    md: "h-10 w-10",
    lg: "h-12 w-12",
};

export function IconPicker({
    value,
    onSelect: onSelect,
    maxIcons: maxIcons = 24,
    startIconId: startIconId = 1,
    columns = 6,
    layout = "grid",
    iconSize: iconSize = "md",
    iconFamily: iconFamily = "agent",
    showClear: showClear = true,
    disabled = false,
    className: className,
}: IconPickerProps) {
    const { t } = useI18n();

    // 生成 icon IDs 列表
    const iconIds = useMemo(() => {
        return Array.from({ length: maxIcons }, (_, i) => String(startIconId + i));
    }, [maxIcons, startIconId]);

    const gridCols = cn(
        "gap-2",
        columns === 4 && "grid-cols-4",
        columns === 6 && "grid-cols-6",
        columns === 8 && "grid-cols-8",
    );

    return (
        <div className={cn("flex flex-col gap-3", className)}>
            {/* 清除按钮 */}
            {showClear && value ? (
                <button
                    className="inline-flex items-center gap-1.5 text-[12px] font-semibold text-(--text-muted) hover:text-(--text-default) transition"
                    onClick={() => onSelect("")}
                    type="button"
                    disabled={disabled}
                >
                    <X className="h-3.5 w-3.5" />
                    {t("common.clear")}
                </button>
            ) : null}

            {/* 图标网格 */}
            <div
                className={cn(
                    layout === "row"
                        ? "soft-scrollbar flex gap-2 overflow-x-auto overflow-y-hidden pb-1"
                        : cn("grid", gridCols),
                )}
            >
                {iconIds.map((iconId) => {
                    const isSelected = value === iconId;
                    const iconPath = `/icon/${iconFamily}/${iconId}.png`;

                    return (
                        <button
                            key={iconId}
                            className={cn(
                                "relative inline-flex items-center justify-center overflow-hidden rounded-[12px] transition-[background,transform,border-color,box-shadow] duration-(--motion-duration-fast) cursor-pointer",
                                ICON_SIZE_MAP[iconSize],
                                layout === "row" && "shrink-0",
                                isSelected
                                    ? "bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] border border-(--primary) shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_16%,transparent)]"
                                    : "border border-(--surface-inset-border) bg-transparent hover:bg-(--surface-interactive-hover-background) hover:-translate-y-[1px]",
                                disabled && "cursor-not-allowed opacity-50",
                            )}
                            onClick={() => !disabled && onSelect(iconId)}
                            type="button"
                            disabled={disabled}
                            title={`icon-${iconId}`}
                        >
                            <img
                                alt={`icon-${iconId}`}
                                className="h-full w-full rounded-[inherit] object-cover"
                                crossOrigin="anonymous"
                                src={iconPath}
                            />
                        </button>
                    );
                })}
            </div>
        </div>
    );
}
