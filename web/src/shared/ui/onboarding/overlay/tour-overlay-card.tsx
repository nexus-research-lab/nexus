import { forwardRef } from "react";
import {
  Bot,
  Hash,
  Puzzle,
  Users2,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type {
  OnboardingTourStep,
  OnboardingTourStepItem,
} from "../tour-provider";
import type { TourPlacement } from "./tour-overlay-geometry";

const TOUR_ITEM_ICONS: Record<OnboardingTourStepItem["icon"], LucideIcon> = {
  bot: Bot,
  hash: Hash,
  puzzle: Puzzle,
  users: Users2,
};

interface TourOverlayCardProps {
  isLastStep: boolean;
  onClose: (options?: { completed?: boolean }) => void;
  onNext: () => void;
  onPrevious: () => void;
  placement: TourPlacement;
  step: OnboardingTourStep;
  stepIndex: number;
  stepCount: number;
}

export const TourOverlayCard = forwardRef<
  HTMLDivElement,
  TourOverlayCardProps
>(function TourOverlayCard({
  isLastStep,
  onClose,
  onNext,
  onPrevious,
  placement,
  step,
  stepCount,
  stepIndex,
}, ref) {
  const { t } = useI18n();

  return (
    <div
      className="surface-popover relative max-h-[calc(100vh-80px)] w-[min(344px,calc(100vw-32px))] overflow-y-auto rounded-[12px] border px-4 py-3.5 shadow-[0_14px_32px_color-mix(in_srgb,var(--shadow-color)_14%,transparent)]"
      ref={ref}
    >
      {step.image ? (
        <TourStepIllustration
          centered={placement === "center"}
          src={step.image}
          title={step.title}
        />
      ) : null}

      <div className="flex items-start justify-between gap-4">
        <h3 className="mt-0.5 min-w-0 text-[18px] font-semibold tracking-tight text-(--text-strong)">
          {step.title}
        </h3>
        <button
          className="shrink-0 rounded-full px-2 py-1 text-[11px] font-medium text-(--text-muted) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
          onClick={() => onClose({ completed: true })}
          type="button"
        >
          {t("common.skip")}
        </button>
      </div>

      <p className="mt-2.5 text-[13px] leading-6 text-(--text-default)">
        {step.description}
      </p>

      {step.items?.length ? <TourStepItems items={step.items} /> : null}

      <div className="mt-3.5 flex items-center justify-between gap-3">
        <span className="text-[11px] font-medium tabular-nums text-(--text-muted)">
          {stepIndex + 1} / {stepCount}
        </span>
        <div className="flex items-center gap-2">
          <button
            className="rounded-full border border-(--divider-subtle-color) px-3 py-1.5 text-[11px] font-medium text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
            disabled={stepIndex === 0}
            onClick={onPrevious}
            type="button"
          >
            {t("common.back")}
          </button>
          <button
            className="rounded-full bg-(--primary) px-3 py-1.5 text-[11px] font-medium text-white transition-[transform,opacity] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:opacity-92"
            onClick={isLastStep ? () => onClose({ completed: true }) : onNext}
            type="button"
          >
            {isLastStep ? t("common.finish") : t("common.next")}
          </button>
        </div>
      </div>
    </div>
  );
});

function TourStepIllustration({
  centered,
  src,
  title,
}: {
  centered: boolean;
  src: string;
  title: string;
}) {
  return (
    <div className="mb-3 rounded-[12px] border border-(--divider-subtle-color) bg-transparent p-2.5">
      <div className="relative overflow-hidden rounded-[10px] border border-(--divider-subtle-color) bg-transparent">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_20%_18%,rgba(255,255,255,0.76),transparent_36%),radial-gradient(circle_at_82%_84%,rgba(132,146,255,0.12),transparent_34%),linear-gradient(180deg,rgba(255,255,255,0.18),transparent_68%)]" />
        <div className="pointer-events-none absolute inset-x-0 bottom-0 h-8 bg-[linear-gradient(180deg,transparent,rgba(132,146,255,0.06))]" />
        <img
          alt={title}
          className={cn(
            "relative z-10 mx-auto w-full object-contain px-2 py-2.5 [image-rendering:auto]",
            "drop-shadow-[0_10px_18px_rgba(87,98,173,0.10)] mix-blend-multiply",
            centered ? "h-[132px]" : "h-[112px]",
          )}
          src={src}
        />
      </div>
    </div>
  );
}

function TourStepItems({ items }: { items: OnboardingTourStepItem[] }) {
  return (
    <div className="mt-2.5 flex flex-col gap-1.5">
      {items.map((item) => {
        const Icon = TOUR_ITEM_ICONS[item.icon];
        return (
          <div
            className="flex items-center gap-2 rounded-[10px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)] px-2.5 py-1.5"
            key={item.text}
          >
            <Icon className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="text-[12px] leading-5 text-(--text-muted)">
              {item.text}
            </span>
          </div>
        );
      })}
    </div>
  );
}
