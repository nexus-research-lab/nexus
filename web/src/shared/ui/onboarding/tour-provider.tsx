"use client";

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { Hash, Puzzle, Users2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  read_completed_tours,
  reset_all_tour_state,
  write_completed_tours,
} from "@/shared/ui/onboarding/tour-state";

type TourPlacement = "top" | "right" | "bottom" | "left" | "center";
type TourStickerPlacement = "hang" | "perch" | "peek" | "point" | "hold";

interface TourStickerAsset {
  src: string;
  placement: TourStickerPlacement;
}

export interface OnboardingTourStepItem {
  icon: "users" | "hash" | "puzzle";
  text: string;
}

export interface OnboardingTourStep {
  id: string;
  title: string;
  description: string;
  target?: string;
  placement?: TourPlacement;
  items?: OnboardingTourStepItem[];
  image?: string;
}

export interface OnboardingTourDefinition {
  id: string;
  steps: OnboardingTourStep[];
}

interface ActiveTourState {
  tour_id: string;
  step_index: number;
}

interface OnboardingTourContextValue {
  register_tour: (tour: OnboardingTourDefinition) => void;
  unregister_tour: (tour_id: string) => void;
  start_tour: (tour_id: string) => void;
  close_tour: (options?: { completed?: boolean }) => void;
  next_step: () => void;
  previous_step: () => void;
  has_completed_tour: (tour_id: string) => boolean;
  is_tour_registered: (tour_id: string) => boolean;
  reset_all_tours: () => void;
  active_tour_id: string | null;
  reset_version: number;
}

const ONBOARDING_TOUR_CONTEXT = createContext<OnboardingTourContextValue | null>(null);
const TOUR_STICKERS: TourStickerAsset[] = [
  { src: "/nexus/stickers/card-top.png", placement: "perch" },
  { src: "/nexus/stickers/hanging.png", placement: "hang" },
  { src: "/nexus/stickers/peek-right.png", placement: "peek" },
  { src: "/nexus/stickers/pointing.png", placement: "point" },
  { src: "/nexus/stickers/holding-card.png", placement: "hold" },
];

function clamp_step_index(step_index: number, steps_count: number): number {
  if (steps_count <= 0) {
    return 0;
  }
  return Math.max(0, Math.min(step_index, steps_count - 1));
}

interface PopoverPosition {
  top: number;
  left: number;
}

interface PopoverSize {
  width: number;
  height: number;
}

function estimate_card_height(step?: OnboardingTourStep): number {
  if (!step) return 180;
  let height = 104;
  if (step.image) height += 136;
  if (step.description) height += 24;
  if (step.items?.length) height += step.items.length * 34;
  return height;
}

function clamp_popover_top_with_clearance(
  top: number,
  card_height: number,
  viewport_height: number,
  top_clearance: number,
): number {
  const bottom_limit = viewport_height - card_height - 16;
  if (bottom_limit < top_clearance) {
    return Math.max(16, bottom_limit);
  }
  return Math.max(top_clearance, Math.min(top, bottom_limit));
}

function clamp_popover_left(left: number, card_width: number, viewport_width: number): number {
  return Math.max(16, Math.min(left, viewport_width - card_width - 16));
}

function resolve_tour_sticker(
  step_index: number,
  placement: TourPlacement,
): TourStickerAsset {
  if (placement === "center") {
    return TOUR_STICKERS[0];
  }
  if (placement === "left") {
    return TOUR_STICKERS[2];
  }
  return TOUR_STICKERS[(step_index + 1) % TOUR_STICKERS.length];
}

function TourStepSticker({ sticker }: { sticker: TourStickerAsset }) {
  const sticker_class_name: Record<TourStickerPlacement, string> = {
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
        sticker_class_name[sticker.placement],
      )}
      src={sticker.src}
    />
  );
}

function get_sticker_top_clearance(sticker: TourStickerAsset): number {
  if (sticker.placement === "hang" || sticker.placement === "perch" || sticker.placement === "point") {
    return 72;
  }
  return 16;
}

function TourStepIllustration({
  src,
  title,
  is_center_step,
}: {
  src: string;
  title: string;
  is_center_step: boolean;
}) {
  return (
    <div className="mb-3 rounded-[18px] border border-[color:color-mix(in_srgb,var(--primary)_10%,white)] bg-[linear-gradient(180deg,rgba(255,255,255,0.94),rgba(246,248,255,0.94)),radial-gradient(circle_at_top_left,rgba(132,146,255,0.12),transparent_54%)] p-2.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.66),0_10px_28px_color-mix(in_srgb,var(--shadow-color)_7%,transparent)]">
      <div className="relative overflow-hidden rounded-[14px] border border-white/80 bg-[linear-gradient(180deg,#f6f5fb,#efedf8)]">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_20%_18%,rgba(255,255,255,0.76),transparent_36%),radial-gradient(circle_at_82%_84%,rgba(132,146,255,0.12),transparent_34%),linear-gradient(180deg,rgba(255,255,255,0.18),transparent_68%)]" />
        <div className="pointer-events-none absolute inset-x-0 bottom-0 h-8 bg-[linear-gradient(180deg,transparent,rgba(132,146,255,0.06))]" />
        <img
          alt={title}
          className={cn(
            "relative z-10 mx-auto w-full object-contain px-2 py-2.5 [image-rendering:auto]",
            "drop-shadow-[0_10px_18px_rgba(87,98,173,0.10)] mix-blend-multiply",
            is_center_step ? "h-[132px]" : "h-[112px]",
          )}
          src={src}
        />
      </div>
    </div>
  );
}

function get_popover_position(
  placement: TourPlacement,
  target_rect: DOMRect | null,
  viewport_width: number,
  viewport_height: number,
  popover_size: PopoverSize,
  top_clearance: number,
): PopoverPosition {
  const card_width = Math.min(popover_size.width, viewport_width - 32);
  const card_height = Math.min(popover_size.height, viewport_height - 32);
  const gutter = 16;

  if (!target_rect || placement === "center") {
    return {
      top: clamp_popover_top_with_clearance(
        viewport_height / 2 - card_height / 2,
        card_height,
        viewport_height,
        top_clearance,
      ),
      left: clamp_popover_left(viewport_width / 2 - card_width / 2, card_width, viewport_width),
    };
  }

  switch (placement) {
    case "left":
      return {
        top: clamp_popover_top_with_clearance(
          target_rect.top + target_rect.height / 2 - card_height / 2,
          card_height,
          viewport_height,
          top_clearance,
        ),
        left: clamp_popover_left(target_rect.left - card_width - gutter, card_width, viewport_width),
      };
    case "top":
      return {
        top: clamp_popover_top_with_clearance(
          target_rect.top - card_height - gutter,
          card_height,
          viewport_height,
          top_clearance,
        ),
        left: clamp_popover_left(
          target_rect.left + target_rect.width / 2 - card_width / 2,
          card_width,
          viewport_width,
        ),
      };
    case "bottom":
      return {
        top: clamp_popover_top_with_clearance(
          target_rect.bottom + gutter,
          card_height,
          viewport_height,
          top_clearance,
        ),
        left: clamp_popover_left(
          target_rect.left + target_rect.width / 2 - card_width / 2,
          card_width,
          viewport_width,
        ),
      };
    case "right":
    default: {
      const raw_top = target_rect.top + target_rect.height / 2 - card_height / 2;
      return {
        top: clamp_popover_top_with_clearance(
          raw_top,
          card_height,
          viewport_height,
          top_clearance,
        ),
        left: clamp_popover_left(
          target_rect.right + gutter,
          card_width,
          viewport_width,
        ),
      };
    }
  }
}

function OnboardingTourOverlay({
  tour,
  step_index,
  on_close,
  on_next,
  on_previous,
}: {
  tour: OnboardingTourDefinition;
  step_index: number;
  on_close: (options?: { completed?: boolean }) => void;
  on_next: () => void;
  on_previous: () => void;
}) {
  const { t } = useI18n();
  const step = tour.steps[step_index];
  const [target_rect, set_target_rect] = useState<DOMRect | null>(null);
  const card_ref = useRef<HTMLDivElement | null>(null);
  const [popover_size, set_popover_size] = useState<PopoverSize>({
    width: Math.min(344, typeof window === "undefined" ? 344 : window.innerWidth - 32),
    height: estimate_card_height(step),
  });

  useEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const update_target_rect = () => {
      if (!step?.target) {
        set_target_rect(null);
        return;
      }

      const target_element = document.querySelector<HTMLElement>(
        `[data-tour-anchor="${step.target}"]`,
      );
      if (!target_element) {
        set_target_rect(null);
        return;
      }

      target_element.scrollIntoView({
        block: "nearest",
        inline: "nearest",
      });
      set_target_rect(target_element.getBoundingClientRect());
    };

    update_target_rect();
    window.addEventListener("resize", update_target_rect);
    window.addEventListener("scroll", update_target_rect, true);

    return () => {
      window.removeEventListener("resize", update_target_rect);
      window.removeEventListener("scroll", update_target_rect, true);
    };
  }, [step?.target]);

  useLayoutEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }

    const element = card_ref.current;
    if (!element) {
      return undefined;
    }

    const update_size = () => {
      const rect = element.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0) {
        return;
      }
      set_popover_size((current_size) => {
        if (
          Math.abs(current_size.width - rect.width) < 1
          && Math.abs(current_size.height - rect.height) < 1
        ) {
          return current_size;
        }
        return {
          width: rect.width,
          height: rect.height,
        };
      });
    };

    update_size();

    const resize_observer = new ResizeObserver(() => {
      update_size();
    });
    resize_observer.observe(element);
    window.addEventListener("resize", update_size);

    return () => {
      resize_observer.disconnect();
      window.removeEventListener("resize", update_size);
    };
  }, [step]);

  if (typeof document === "undefined" || !step) {
    return null;
  }

  const placement = step.placement ?? (step.target ? "right" : "center");
  const sticker = resolve_tour_sticker(step_index, placement);
  const popover_position = get_popover_position(
    placement,
    target_rect,
    window.innerWidth,
    window.innerHeight,
    popover_size,
    get_sticker_top_clearance(sticker),
  );
  const is_last_step = step_index >= tour.steps.length - 1;

  const overlay = (
    <div className="fixed inset-0 z-[11000]">
      <div
        className="absolute inset-0 bg-[rgba(11,16,24,0.46)] backdrop-blur-[1px]"
        onClick={() => on_close()}
      />

      {target_rect ? (
        <div
          className="pointer-events-none absolute rounded-[22px] border border-[color:color-mix(in_srgb,var(--primary)_34%,white)] shadow-[0_0_0_9999px_rgba(11,16,24,0.22),0_24px_64px_color-mix(in_srgb,var(--primary)_18%,transparent)] transition-[top,left,width,height] duration-(--motion-duration-fast)"
          style={{
            top: target_rect.top - 6,
            left: target_rect.left - 6,
            width: target_rect.width + 12,
            height: target_rect.height + 12,
          }}
        />
      ) : null}

      <div
        className="absolute"
        style={{
          top: popover_position.top,
          left: popover_position.left,
        }}
      >
        <div className="relative">
          <TourStepSticker sticker={sticker} />
          <div
            ref={card_ref}
            className={cn(
              "surface-popover relative max-h-[calc(100vh-80px)] w-[min(344px,calc(100vw-32px))] overflow-y-auto rounded-[24px] border px-4 py-3.5 shadow-[0_22px_58px_color-mix(in_srgb,var(--shadow-color)_16%,transparent)]",
            )}
          >
            {step.image ? (
              <TourStepIllustration
                is_center_step={placement === "center"}
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
                onClick={() => on_close()}
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
                {step_index + 1} / {tour.steps.length}
              </span>
              <div className="flex items-center gap-2">
                <button
                  className="rounded-full border border-(--divider-subtle-color) px-3 py-1.5 text-[11px] font-medium text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
                  disabled={step_index === 0}
                  onClick={on_previous}
                  type="button"
                >
                  {t("common.back")}
                </button>
                <button
                  className="rounded-full bg-(--primary) px-3 py-1.5 text-[11px] font-medium text-white transition-[transform,opacity] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:opacity-92"
                  onClick={is_last_step ? () => on_close({ completed: true }) : on_next}
                  type="button"
                >
                  {is_last_step ? t("common.finish") : t("common.next")}
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

export function OnboardingTourProvider({ children }: { children: ReactNode }) {
  const tours_ref = useRef<Record<string, OnboardingTourDefinition>>({});
  const [completed_tours, set_completed_tours] = useState<Record<string, boolean>>(
    () => read_completed_tours(),
  );
  const [active_tour, set_active_tour] = useState<ActiveTourState | null>(null);
  const [reset_version, set_reset_version] = useState(0);

  const register_tour = useCallback((tour: OnboardingTourDefinition) => {
    tours_ref.current[tour.id] = tour;
  }, []);

  const unregister_tour = useCallback((tour_id: string) => {
    delete tours_ref.current[tour_id];
  }, []);

  const start_tour = useCallback((tour_id: string) => {
    const tour = tours_ref.current[tour_id];
    if (!tour || tour.steps.length === 0) {
      return;
    }

    set_active_tour({
      tour_id,
      step_index: 0,
    });
  }, []);

  const close_tour = useCallback((options?: { completed?: boolean }) => {
    set_active_tour((current_tour) => {
      if (!current_tour) {
        return null;
      }

      if (options?.completed) {
        set_completed_tours((previous) => {
          const next_value = {
            ...previous,
            [current_tour.tour_id]: true,
          };
          write_completed_tours(next_value);
          return next_value;
        });
      }

      return null;
    });
  }, []);

  const next_step = useCallback(() => {
    set_active_tour((current_tour) => {
      if (!current_tour) {
        return null;
      }
      const current_definition = tours_ref.current[current_tour.tour_id];
      if (!current_definition) {
        return null;
      }
      const next_index = clamp_step_index(
        current_tour.step_index + 1,
        current_definition.steps.length,
      );
      return {
        ...current_tour,
        step_index: next_index,
      };
    });
  }, []);

  const previous_step = useCallback(() => {
    set_active_tour((current_tour) => {
      if (!current_tour) {
        return null;
      }
      const current_definition = tours_ref.current[current_tour.tour_id];
      if (!current_definition) {
        return null;
      }
      const next_index = clamp_step_index(
        current_tour.step_index - 1,
        current_definition.steps.length,
      );
      return {
        ...current_tour,
        step_index: next_index,
      };
    });
  }, []);

  const has_completed_tour = useCallback((tour_id: string) => {
    return Boolean(completed_tours[tour_id]);
  }, [completed_tours]);

  const is_tour_registered = useCallback((tour_id: string) => {
    return Boolean(tours_ref.current[tour_id]);
  }, []);

  const reset_all_tours = useCallback(() => {
    reset_all_tour_state();
    set_completed_tours({});
    set_active_tour(null);
    set_reset_version((current_value) => current_value + 1);
  }, []);

  const context_value = useMemo<OnboardingTourContextValue>(() => ({
    register_tour,
    unregister_tour,
    start_tour,
    close_tour,
    next_step,
    previous_step,
    has_completed_tour,
    is_tour_registered,
    reset_all_tours,
    active_tour_id: active_tour?.tour_id ?? null,
    reset_version,
  }), [
    active_tour?.tour_id,
    close_tour,
    has_completed_tour,
    is_tour_registered,
    next_step,
    previous_step,
    register_tour,
    reset_version,
    reset_all_tours,
    start_tour,
    unregister_tour,
  ]);

  const active_tour_definition = active_tour
    ? tours_ref.current[active_tour.tour_id] ?? null
    : null;

  return (
    <ONBOARDING_TOUR_CONTEXT.Provider value={context_value}>
      {children}
      {active_tour_definition && active_tour ? (
        <OnboardingTourOverlay
          on_close={close_tour}
          on_next={next_step}
          on_previous={previous_step}
          step_index={active_tour.step_index}
          tour={active_tour_definition}
        />
      ) : null}
    </ONBOARDING_TOUR_CONTEXT.Provider>
  );
}

function TourItemIcon({ name }: { name: OnboardingTourStepItem["icon"] }) {
  const className = "h-3.5 w-3.5 shrink-0 text-(--icon-muted)";
  if (name === "users") return <Users2 className={className} />;
  if (name === "hash") return <Hash className={className} />;
  return <Puzzle className={className} />;
}

export function useOnboardingTour() {
  const context = useContext(ONBOARDING_TOUR_CONTEXT);
  if (!context) {
    throw new Error("useOnboardingTour 必须在 OnboardingTourProvider 内部使用");
  }
  return context;
}
