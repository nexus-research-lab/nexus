import { forwardRef } from "react";
import {
  Bot,
  Hash,
  Puzzle,
  Users2,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { cn } from "@/shared/ui/class-name";
import type {
  OnboardingTourStep,
  OnboardingTourStepItem,
} from "../tour-contract";
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
      className="surface-popover relative max-h-[calc(100vh-64px)] w-[min(336px,calc(100vw-32px))] overflow-y-auto rounded-[10px] px-4 py-3 shadow-[0_10px_24px_color-mix(in_srgb,var(--shadow-color)_12%,transparent)]"
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
        <h3 className="mt-0.5 min-w-0 text-[16px] font-semibold tracking-tight text-(--text-strong)">
          {step.title}
        </h3>
        <button
          className="shrink-0 px-1 py-0.5 text-[11px] font-medium text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)"
          onClick={() => onClose({ completed: true })}
          type="button"
        >
          {t("common.skip")}
        </button>
      </div>

      <p className="mt-2 text-[12px] leading-5 text-(--text-default)">
        {step.description}
      </p>

      {step.items?.length ? <TourStepItems items={step.items} /> : null}

      <div className="mt-3 border-t border-(--divider-subtle-color) pt-3 flex items-center justify-between gap-3">
        <span className="text-[11px] font-medium tabular-nums text-(--text-muted)">
          {stepIndex + 1} / {stepCount}
        </span>
        <div className="flex items-center gap-2">
          <button
            className={getUiButtonClassName(
              { size: "xs", tone: "default", variant: "text" },
              "font-medium",
            )}
            disabled={stepIndex === 0}
            onClick={onPrevious}
            type="button"
          >
            {t("common.back")}
          </button>
          <button
            className={getUiButtonClassName(
              { size: "xs", tone: "primary", variant: "surface" },
              "font-medium",
            )}
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
    <div className="mb-3 flex items-center justify-center border-b border-(--divider-subtle-color) pb-3">
      <img
        alt={title}
        className={cn(
          "w-auto object-contain [image-rendering:auto]",
          centered ? "h-[104px]" : "h-[84px]",
        )}
        src={src}
      />
    </div>
  );
}

function TourStepItems({ items }: { items: OnboardingTourStepItem[] }) {
  return (
    <div className="mt-2.5 divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
      {items.map((item) => {
        const Icon = TOUR_ITEM_ICONS[item.icon];
        return (
          <div
            className="flex items-center gap-2 py-2"
            key={item.text}
          >
            <Icon className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
            <span className="text-[12px] leading-5 text-(--text-default)">
              {item.text}
            </span>
          </div>
        );
      })}
    </div>
  );
}
