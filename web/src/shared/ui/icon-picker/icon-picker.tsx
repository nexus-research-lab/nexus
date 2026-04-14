"use client";

/**
 * 通用图标选择器
 * 
 * 用于 Room Avatar、Agent Avatar 等场景。
 * 支持 1-24 号图标 (icon/{number}.png)
 */

import { X } from "lucide-react";
import { useMemo } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";

interface IconPickerProps {
    value?: string; // e.g. "13"
    on_select: (icon_id: string) => void;
    max_icons?: number; // 默认 24
    start_icon_id?: number; // 默认 1
    columns?: number; // 默认 4
    layout?: "grid" | "row";
    icon_size?: "sm" | "md" | "lg"; // 默认 md
    show_clear?: boolean;
    disabled?: boolean;
    class_name?: string;
}

const ICON_SIZE_MAP = {
    sm: "h-8 w-8",
    md: "h-10 w-10",
    lg: "h-12 w-12",
};

export function IconPicker({
    value,
    on_select,
    max_icons = 24,
    start_icon_id = 1,
    columns = 6,
    layout = "grid",
    icon_size = "md",
    show_clear = true,
    disabled = false,
    class_name,
}: IconPickerProps) {
    const { t } = useI18n();

    // 生成 icon IDs 列表
    const icon_ids = useMemo(() => {
        return Array.from({ length: max_icons }, (_, i) => String(start_icon_id + i));
    }, [max_icons, start_icon_id]);

    const grid_cols = cn(
        "gap-2",
        columns === 4 && "grid-cols-4",
        columns === 6 && "grid-cols-6",
        columns === 8 && "grid-cols-8",
    );

    return (
        <div className={cn("flex flex-col gap-3", class_name)}>
            {/* 清除按钮 */}
            {show_clear && value ? (
                <button
                    className="inline-flex items-center gap-1.5 text-[12px] font-semibold text-(--text-muted) hover:text-(--text-default) transition"
                    onClick={() => on_select("")}
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
                        : cn("grid", grid_cols),
                )}
            >
                {icon_ids.map((icon_id) => {
                    const is_selected = value === icon_id;
                    const icon_path = `/icon/${icon_id}.png`;

                    return (
                        <button
                            key={icon_id}
                            className={cn(
                                "relative inline-flex items-center justify-center rounded-[12px] transition-[background,transform,border-color,box-shadow] duration-(--motion-duration-fast) cursor-pointer",
                                ICON_SIZE_MAP[icon_size],
                                layout === "row" && "shrink-0",
                                is_selected
                                    ? "bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] border border-(--primary) shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_16%,transparent)]"
                                    : "bg-(--surface-inset-background) border border-(--surface-inset-border) hover:bg-(--surface-card-background) hover:-translate-y-[1px]",
                                disabled && "cursor-not-allowed opacity-50",
                            )}
                            onClick={() => !disabled && on_select(icon_id)}
                            type="button"
                            disabled={disabled}
                            title={`icon-${icon_id}`}
                        >
                            <img
                                alt={`icon-${icon_id}`}
                                className="h-full w-full object-contain"
                                crossOrigin="anonymous"
                                src={icon_path}
                            />
                        </button>
                    );
                })}
            </div>
        </div>
    );
}
