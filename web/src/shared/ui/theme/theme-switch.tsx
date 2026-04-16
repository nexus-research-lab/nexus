/**
 * =====================================================
 * @File   : theme-switch.tsx
 * @Date   : 2026-04-04 18:06
 * @Author : leemysw
 * 2026-04-04 18:06   Create
 * =====================================================
 */

"use client";

import { CloudRain, MoonStar, Sun, SunMedium } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { Theme, useTheme } from "@/shared/theme/theme-context";
import { SegmentedPill } from "@/shared/ui/segmented-pill";

const THEME_META: Record<Theme, { icon: typeof Sun }> = {
  light: { icon: SunMedium },
  dark: { icon: MoonStar },
  sunny: { icon: Sun },
  rain: { icon: CloudRain },
};

export function ThemeSwitch({
  class_name,
  density,
  show_icon = true,
  stretch,
}: {
  class_name?: string;
  density?: "default" | "compact";
  show_icon?: boolean;
  stretch?: boolean;
}) {
  const { theme, set_theme } = useTheme();
  const { t } = useI18n();

  const options: { value: Theme; label: string }[] = [
    { value: "light", label: t("theme.light") },
    { value: "dark", label: t("theme.dark") },
    { value: "sunny", label: t("theme.sunny") },
    { value: "rain", label: t("theme.rain") },
  ];

  const ActiveIcon = THEME_META[theme].icon;

  return (
    <SegmentedPill
      class_name={class_name}
      density={density}
      icon={show_icon ? ActiveIcon : undefined}
      on_change={set_theme}
      options={options}
      stretch={stretch}
      title={t("theme.switch_title")}
      value={theme}
    />
  );
}
