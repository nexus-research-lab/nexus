"use client";

import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { Bot, Hash, Puzzle, Users2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  OnboardingTourDefinition,
  OnboardingTourStep,
  OnboardingTourStepItem,
} from "@/shared/ui/onboarding/tour-provider";

type TourPlacement = "top" | "right" | "bottom" | "left" | "center";
type TourStickerPlacement = "hang" | "perch" | "peek" | "point" | "hold";

interface TourStickerAsset {
  src: string;
  placement: TourStickerPlacement;
}

interface PopoverPosition {
  top: number;
  left: number;
}

interface PopoverSize {
  width: number;
  height: number;
}

const TOUR_STICKERS: TourStickerAsset[] = [
  { src: "/nexus/stickers/card-top.png", placement: "perch" },
  { src: "/nexus/stickers/hanging.png", placement: "hang" },
  { src: "/nexus/stickers/peek-right.png", placement: "peek" },
  { src: "/nexus/stickers/pointing.png", placement: "point" },
  { src: "/nexus/stickers/holding-card.png", placement: "hold" },
];

function estimateCardHeight(step?: OnboardingTourStep): number {
  if (!step) return 180;
  let height = 104;
  if (step.image) height += 136;
  if (step.description) height += 24;
  if (step.items?.length) height += step.items.length * 34;
  return height;
}

function clampPopoverTopWithClearance(
  top: number,
  cardHeight: number,
  viewportHeight: number,
  topClearance: number,
): number {
  const bottomLimit = viewportHeight - cardHeight - 16;
  if (bottomLimit < topClearance) {
    return Math.max(16, bottomLimit);
  }
  return Math.max(topClearance, Math.min(top, bottomLimit));
}

function clampPopoverLeft(left: number, cardWidth: number, viewportWidth: number): number {
  return Math.max(16, Math.min(left, viewportWidth - cardWidth - 16));
}

function resolveTourSticker(
  stepIndex: number,
  placement: TourPlacement,
): TourStickerAsset {
  if (placement === "center") {
    return TOUR_STICKERS[0];
  }
  if (placement === "left") {
    return TOUR_STICKERS[2];
  }
  return TOUR_STICKERS[(stepIndex + 1) % TOUR_STICKERS.length];
}

function TourStepSticker({ sticker }: { sticker: TourStickerAsset }) {
  const stickerClassName: Record<TourStickerPlacement, string> = {
    hang: "-top-12 right-7 h-[72px] w-auto",
    perch: "-top-10 left-14 h-[74px] w-auto -translate-x-1/2",
    peek: "top-16 -left-10 h-[82px] w-auto",
    point: "-top-[52px] right-4 h-[72px] w-auto",
    hold: "top-1/2 -right-10 h-[82px] w-auto -translate-y-1/2",
  };

  return (
    <img
      alt=""
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute z-20 select-none drop-shadow-[0_14px_20px_rgba(68,74,120,0.12)] max-[520px]:hidden",
        stickerClassName[sticker.placement],
      )}
      src={sticker.src}
    />
  );
}

function getStickerTopClearance(sticker: TourStickerAsset): number {
  if (sticker.placement === "hang" || sticker.placement === "perch" || sticker.placement === "point") {
    return 72;
  }
  return 16;
}

function TourStepIllustration({
  src,
  title,
  isCenterStep: isCenterStep,
}: {
  src: string;
  title: string;
  isCenterStep: boolean;
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
            isCenterStep ? "h-[132px]" : "h-[112px]",
          )}
          src={src}
        />
      </div>
    </div>
  );
}

function getPopoverPosition(
  placement: TourPlacement,
  targetRect: DOMRect | null,
  viewportWidth: number,
  viewportHeight: number,
  popoverSize: PopoverSize,
  topClearance: number,
): PopoverPosition {
  const cardWidth = Math.min(popoverSize.width, viewportWidth - 32);
  const cardHeight = Math.min(popoverSize.height, viewportHeight - 32);
  const gutter = 16;

  if (!targetRect || placement === "center") {
    return {
      top: clampPopoverTopWithClearance(
        viewportHeight / 2 - cardHeight / 2,
        cardHeight,
        viewportHeight,
        topClearance,
      ),
      left: clampPopoverLeft(viewportWidth / 2 - cardWidth / 2, cardWidth, viewportWidth),
    };
  }

  switch (placement) {
    case "left":
      return {
        top: clampPopoverTopWithClearance(
          targetRect.top + targetRect.height / 2 - cardHeight / 2,
          cardHeight,
          viewportHeight,
          topClearance,
        ),
        left: clampPopoverLeft(targetRect.left - cardWidth - gutter, cardWidth, viewportWidth),
      };
    case "top":
      return {
        top: clampPopoverTopWithClearance(
          targetRect.top - cardHeight - gutter,
          cardHeight,
          viewportHeight,
          topClearance,
        ),
        left: clampPopoverLeft(
          targetRect.left + targetRect.width / 2 - cardWidth / 2,
          cardWidth,
          viewportWidth,
        ),
      };
    case "bottom":
      return {
        top: clampPopoverTopWithClearance(
          targetRect.bottom + gutter,
          cardHeight,
          viewportHeight,
          topClearance,
        ),
        left: clampPopoverLeft(
          targetRect.left + targetRect.width / 2 - cardWidth / 2,
          cardWidth,
          viewportWidth,
        ),
      };
    case "right":
    default: {
      const rawTop = targetRect.top + targetRect.height / 2 - cardHeight / 2;
      return {
        top: clampPopoverTopWithClearance(
          rawTop,
          cardHeight,
          viewportHeight,
          topClearance,
        ),
        left: clampPopoverLeft(
          targetRect.right + gutter,
          cardWidth,
          viewportWidth,
        ),
      };
    }
  }
}

export function OnboardingTourOverlay({
  tour,
  stepIndex: stepIndex,
  onClose: onClose,
  onNext: onNext,
  onPrevious: onPrevious,
}: {
  tour: OnboardingTourDefinition;
  stepIndex: number;
  onClose: (options?: { completed?: boolean }) => void;
  onNext: () => void;
  onPrevious: () => void;
}) {
  const { t } = useI18n();
  const step = tour.steps[stepIndex];
  const [targetRect, setTargetRect] = useState<DOMRect | null>(null);
  const cardRef = useRef<HTMLDivElement | null>(null);
  const [popoverSize, setPopoverSize] = useState<PopoverSize>({
    width: Math.min(344, typeof window === "undefined" ? 344 : window.innerWidth - 32),
    height: estimateCardHeight(step),
  });

  useEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const updateTargetRect = () => {
      if (!step?.target) {
        setTargetRect(null);
        return;
      }

      const targetElement = document.querySelector<HTMLElement>(
        `[data-tour-anchor="${step.target}"]`,
      );
      if (!targetElement) {
        setTargetRect(null);
        return;
      }

      targetElement.scrollIntoView({
        block: "nearest",
        inline: "nearest",
      });
      setTargetRect(targetElement.getBoundingClientRect());
    };

    updateTargetRect();
    window.addEventListener("resize", updateTargetRect);
    window.addEventListener("scroll", updateTargetRect, true);

    return () => {
      window.removeEventListener("resize", updateTargetRect);
      window.removeEventListener("scroll", updateTargetRect, true);
    };
  }, [step?.target]);

  useLayoutEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const element = cardRef.current;
    if (!element) {
      return undefined;
    }

    const updateSize = () => {
      const rect = element.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) {
        return;
      }
      setPopoverSize((currentSize) => {
        if (
          Math.abs(currentSize.width - rect.width) < 1
          && Math.abs(currentSize.height - rect.height) < 1
        ) {
          return currentSize;
        }
        return {
          width: rect.width,
          height: rect.height,
        };
      });
    };

    updateSize();

    const resizeObserver = new ResizeObserver(() => {
      updateSize();
    });
    resizeObserver.observe(element);
    window.addEventListener("resize", updateSize);

    return () => {
      resizeObserver.disconnect();
      window.removeEventListener("resize", updateSize);
    };
  }, [step]);

  if (typeof document === "undefined" || !step) {
    return null;
  }

  const placement = step.placement ?? (step.target ? "right" : "center");
  const sticker = resolveTourSticker(stepIndex, placement);
  const popoverPosition = getPopoverPosition(
    placement,
    targetRect,
    window.innerWidth,
    window.innerHeight,
    popoverSize,
    getStickerTopClearance(sticker),
  );
  const isLastStep = stepIndex >= tour.steps.length - 1;

  const overlay = (
    <div className="fixed inset-0 z-[11000]">
      <div
        className="absolute inset-0 bg-[rgba(11,16,24,0.46)] backdrop-blur-[1px]"
        onClick={() => onClose()}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            onClose();
          }
        }}
        role="presentation"
      />

      {targetRect ? (
        <div
          className="pointer-events-none absolute rounded-[12px] border border-[color:color-mix(in_srgb,var(--primary)_34%,white)] shadow-[0_0_0_9999px_rgba(11,16,24,0.22),0_18px_42px_color-mix(in_srgb,var(--primary)_14%,transparent)] transition-[top,left,width,height] duration-(--motion-duration-fast)"
          style={{
            top: targetRect.top - 6,
            left: targetRect.left - 6,
            width: targetRect.width + 12,
            height: targetRect.height + 12,
          }}
        />
      ) : null}

      <div
        className="absolute"
        style={{
          top: popoverPosition.top,
          left: popoverPosition.left,
        }}
      >
        <div className="relative">
          <TourStepSticker sticker={sticker} />
          <div
            ref={cardRef}
            className={cn(
              "surface-popover relative max-h-[calc(100vh-80px)] w-[min(344px,calc(100vw-32px))] overflow-y-auto rounded-[12px] border px-4 py-3.5 shadow-[0_14px_32px_color-mix(in_srgb,var(--shadow-color)_14%,transparent)]",
            )}
          >
            {step.image ? (
              <TourStepIllustration
                isCenterStep={placement === "center"}
                src={step.image}
                title={step.title}
              />
            ) : null}

            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <h3 className="mt-0.5 text-[18px] font-semibold tracking-tight text-(--text-strong)">
                  {step.title}
                </h3>
              </div>
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

            {step.items && step.items.length > 0 && (
              <div className="mt-2.5 flex flex-col gap-1.5">
                {step.items.map((item) => (
                  <div
                    key={item.text}
                    className="flex items-center gap-2 rounded-[10px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)] px-2.5 py-1.5"
                  >
                    <TourItemIcon name={item.icon} />
                    <span className="text-[12px] leading-5 text-(--text-muted)">{item.text}</span>
                  </div>
                ))}
              </div>
            )}

            <div className="mt-3.5 flex items-center justify-between gap-3">
              <span className="text-[11px] font-medium tabular-nums text-(--text-muted)">
                {stepIndex + 1} / {tour.steps.length}
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
        </div>
      </div>
    </div>
  );

  return createPortal(overlay, document.body);
}

function TourItemIcon({ name }: { name: OnboardingTourStepItem["icon"] }) {
  const className = "h-3.5 w-3.5 shrink-0 text-(--icon-muted)";
  if (name === "bot") return <Bot className={className} />;
  if (name === "users") return <Users2 className={className} />;
  if (name === "hash") return <Hash className={className} />;
  return <Puzzle className={className} />;
}
