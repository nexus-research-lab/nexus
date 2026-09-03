// INPUT: 当前 Tour 步骤、位置、进度和导航/关闭动作。
// OUTPUT: 可滚动的引导卡片、步骤内容与统一 Button 导航。
// POS: Onboarding Tour 卡片视图；不拥有按钮视觉或浮层定位生命周期。

import { forwardRef } from "react";
import {
  Bot,
  Hash,
  Puzzle,
  Users2,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
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
      className="surface-popover surface-radius-lg relative max-h-[calc(100vh-64px)] w-[min(336px,calc(100vw-32px))] overflow-y-auto px-4 py-3"
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
        <h3 className="mt-0.5 min-w-0 text-md font-semibold tracking-tight text-(--text-strong)">
          {step.title}
        </h3>
        <UiButton
          className="shrink-0"
          onClick={() => onClose({ completed: true })}
          size="xs"
          variant="text"
        >
          {t("common.skip")}
        </UiButton>
      </div>

      <p className="mt-2 text-compact leading-5 text-(--text-default)">
        {step.description}
      </p>

      {step.items?.length ? <TourStepItems items={step.items} /> : null}

      <div className="mt-3 border-t border-(--divider-subtle-color) pt-3 flex items-center justify-between gap-3">
        <span className="text-xs font-medium tabular-nums text-(--text-muted)">
          {stepIndex + 1} / {stepCount}
        </span>
        <div className="flex items-center gap-2">
          <UiButton
            disabled={stepIndex === 0}
            onClick={onPrevious}
            size="xs"
            tone="default"
            variant="text"
          >
            {t("common.back")}
          </UiButton>
          <UiButton
            onClick={isLastStep ? () => onClose({ completed: true }) : onNext}
            size="xs"
            tone="primary"
            variant="surface"
          >
            {isLastStep ? t("common.finish") : t("common.next")}
          </UiButton>
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
            <span className="text-compact leading-5 text-(--text-default)">
              {item.text}
            </span>
          </div>
        );
      })}
    </div>
  );
}
