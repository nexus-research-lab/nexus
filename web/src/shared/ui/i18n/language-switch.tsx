/**
 * =====================================================
 * @File   : language-switch.tsx
 * @Date   : 2026-04-04 17:05
 * @Author : leemysw
 * 2026-04-04 17:05   Create
 * =====================================================
 */

"use client";

import { Languages } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { SegmentedPill } from "@/shared/ui/segmented-pill";

export function LanguageSwitch({
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
  const { locale, set_locale, t } = useI18n();

  return (
    <SegmentedPill
      class_name={class_name}
      density={density}
      icon={show_icon ? Languages : undefined}
      on_change={set_locale}
      options={[
        { value: "zh", label: t("language.zh") },
        { value: "en", label: t("language.en") },
      ]}
      stretch={stretch}
      title={t("language.switch_title")}
      value={locale}
    />
  );
}
