// INPUT: 本机侧栏天空偏好与当前时段的天空变体。
// OUTPUT: 以缩略插画呈现的单选卡片组：跟随时间、四个固定变体与关闭；选择即时生效并持久化。
// POS: 外观设置中的侧栏天空选择器；插画与时间表复用侧栏导航 Feature，偏好状态归 shared/theme。
"use client";

import { Clock3, EyeOff } from "lucide-react";
import { useRef, type KeyboardEvent } from "react";

import { SidebarNightSky } from "@/features/navigation/sidebar/view/sidebar-night-sky";
import { useScheduledSkyVariant } from "@/features/navigation/sidebar/use-sidebar-sky-variant";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  SIDEBAR_SKY_VARIANTS,
  useSidebarSkyPreference,
  type SidebarSkyPreference,
  type SidebarSkyVariant,
} from "@/shared/theme/sidebar-sky-preference";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

const SKY_OPTIONS: ReadonlyArray<SidebarSkyPreference> = ["auto", ...SIDEBAR_SKY_VARIANTS, "off"];

const SKY_LABEL_KEYS = {
  auto: "settings.appearance.sky_auto",
  off: "settings.appearance.sky_off",
  dawn: "settings.appearance.sky_dawn",
  day: "settings.appearance.sky_day",
  night: "settings.appearance.sky_night",
  meteor: "settings.appearance.sky_meteor",
} as const satisfies Record<SidebarSkyPreference, string>;

export function SettingsSidebarSkyPicker({ labelledBy }: { labelledBy: string }) {
  const { t } = useI18n();
  const preference = useSidebarSkyPreference((state) => state.preference);
  const setPreference = useSidebarSkyPreference((state) => state.setPreference);
  // 「跟随时间」的缩略图展示此刻时间表会选中的天空。
  const scheduled = useScheduledSkyVariant();
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);

  // 单选组键盘协议：方向键在选项间移动并立即选中，与原生 radio 一致。
  function handleKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    const step = event.key === "ArrowRight" || event.key === "ArrowDown" ? 1
      : event.key === "ArrowLeft" || event.key === "ArrowUp" ? -1 : 0;
    if (!step) return;
    event.preventDefault();
    const current = SKY_OPTIONS.indexOf(preference);
    const next = (current + step + SKY_OPTIONS.length) % SKY_OPTIONS.length;
    setPreference(SKY_OPTIONS[next]);
    optionRefs.current[next]?.focus();
  }

  return (
    <div
      aria-labelledby={labelledBy}
      className="grid grid-cols-3 gap-3 sm:grid-cols-6"
      role="radiogroup"
    >
      {SKY_OPTIONS.map((option, index) => {
        const selected = option === preference;
        const previewVariant: SidebarSkyVariant | null = option === "auto" ? scheduled : option === "off" ? null : option;
        return (
          <button
            aria-checked={selected}
            className="group flex min-w-0 flex-col items-stretch gap-1.5 rounded-(--radius-control-md) text-left outline-none"
            key={option}
            onClick={() => setPreference(option)}
            onKeyDown={handleKeyDown}
            ref={(node) => { optionRefs.current[index] = node; }}
            role="radio"
            tabIndex={selected ? 0 : -1}
            type="button"
          >
            <span
              className={cn(
                "relative block h-12 overflow-hidden rounded-(--radius-control-md) border transition-[box-shadow,border-color]",
                "group-focus-visible:ring-2 group-focus-visible:ring-primary/40",
                selected
                  ? "border-primary ring-2 ring-primary/30"
                  : "border-(--divider-subtle-color) group-hover:border-(--divider-strong-color)",
              )}
            >
              {previewVariant ? (
                <span aria-hidden className="sidebar-sky-preview absolute inset-0" data-sky-variant={previewVariant}>
                  <SidebarNightSky variant={previewVariant} />
                </span>
              ) : (
                <span aria-hidden className="absolute inset-0 flex items-center justify-center bg-(--surface-shell-directory-background) text-(--icon-muted)">
                  <EyeOff className="h-4 w-4" />
                </span>
              )}
              {option === "auto" ? (
                <span aria-hidden className="absolute right-1 top-1 flex h-4 w-4 items-center justify-center rounded-full bg-(--surface-paper-background) text-(--icon-default) shadow-sm">
                  <Clock3 className="h-2.5 w-2.5" />
                </span>
              ) : null}
            </span>
            <span
              className={cn(
                "truncate text-center",
                getUiTypographyClassName({ role: "caption", tone: selected ? "strong" : "soft", weight: selected ? "semibold" : undefined }),
              )}
            >
              {t(SKY_LABEL_KEYS[option])}
            </span>
          </button>
        );
      })}
    </div>
  );
}
