/**
 * =====================================================
 * @File   : sidebar-onboarding-hint.tsx
 * @Date   : 2026-04-15
 * @Author : leemysw
 * 侧边栏新手提示条 — 轻量卡片结构，紧凑但不失层次。
 * =====================================================
 */

"use client";

import { useEffect, useState } from "react";
import type { LucideIcon } from "lucide-react";
import { Hash, Puzzle, Users2, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";

const ONBOARDING_HINT_DISMISSED_KEY = "nexus:sidebar-onboarding-dismissed";

export function SidebarOnboardingHint() {
  const { t } = useI18n();
  const [dismissed, set_dismissed] = useState(true);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const raw = window.localStorage.getItem(ONBOARDING_HINT_DISMISSED_KEY);
    set_dismissed(raw === "true");
  }, []);

  const dismiss = () => {
    set_dismissed(true);
    if (typeof window !== "undefined") {
      window.localStorage.setItem(ONBOARDING_HINT_DISMISSED_KEY, "true");
    }
  };

  if (dismissed) return null;

  return (
    <div className="mb-1.5 rounded-[12px] border border-(--divider-subtle-color) px-2.5 py-2">
      <div className="flex items-start justify-between gap-2">
        <p className="text-[11px] font-bold text-(--text-strong)">
          {t("sidebar.guide_title")}
        </p>
        <button
          aria-label={t("common.close")}
          className="flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-(--icon-muted) opacity-60 transition-[background,color,opacity] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default) hover:opacity-100"
          onClick={dismiss}
          type="button"
        >
          <X className="h-3 w-3" />
        </button>
      </div>

      <div className="mt-1.5 flex flex-col gap-1">
        <HintRow
          icon={Hash}
          text={t("sidebar.guide_rooms")}
        />
        <HintRow
          icon={Users2}
          text={t("sidebar.guide_agents")}
        />
        <HintRow
          icon={Puzzle}
          text={t("sidebar.guide_capabilities")}
        />
      </div>
    </div>
  );
}

function HintRow({ icon: Icon, text }: { icon: LucideIcon; text: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <Icon className="h-3 w-3 shrink-0 text-(--icon-muted)" />
      <span className="text-[11px] leading-relaxed text-(--text-muted)">
        {text}
      </span>
    </div>
  );
}
