"use client";

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { Hash, Puzzle, Users2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";

const TOUR_COMPLETION_STORAGE_KEY = "nexus:onboarding:tours";

type TourPlacement = "top" | "right" | "bottom" | "left" | "center";

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
  active_tour_id: string | null;
}

const ONBOARDING_TOUR_CONTEXT = createContext<OnboardingTourContextValue | null>(null);

function read_completed_tours(): Record<string, boolean> {
  if (typeof window === "undefined") {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(TOUR_COMPLETION_STORAGE_KEY);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as Record<string, boolean>;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function write_completed_tours(next_value: Record<string, boolean>) {
  if (typeof window === "undefined") {
    return;
  }

  window.localStorage.setItem(
    TOUR_COMPLETION_STORAGE_KEY,
    JSON.stringify(next_value),
  );
}

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

function estimate_card_height(step?: OnboardingTourStep): number {
  if (!step) return 180;
  let height = 104;
  if (step.image) height += 136;
  if (step.description) height += 24;
  if (step.items?.length) height += step.items.length * 34;
  return height;
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
  step?: OnboardingTourStep,
): PopoverPosition {
  const card_width = Math.min(344, viewport_width - 32);
  const card_height = Math.min(estimate_card_height(step), viewport_height - 80);
  const gutter = 16;

  if (!target_rect || placement === "center") {
    return {
      top: Math.max(24, Math.min(viewport_height / 2 - card_height / 2, viewport_height - card_height - 24)),
      left: Math.max(16, viewport_width / 2 - card_width / 2),
    };
  }

  switch (placement) {
    case "left":
      return {
        top: Math.max(24, Math.min(target_rect.top + target_rect.height / 2 - card_height / 2, viewport_height - card_height - 24)),
        left: Math.max(16, target_rect.left - card_width - gutter),
      };
    case "top":
      return {
        top: Math.max(24, target_rect.top - card_height - gutter),
        left: Math.min(
          Math.max(16, target_rect.left + target_rect.width / 2 - card_width / 2),
          viewport_width - card_width - 16,
        ),
      };
    case "bottom":
      return {
        top: Math.min(
          target_rect.bottom + gutter,
          viewport_height - card_height - 24,
        ),
        left: Math.min(
          Math.max(16, target_rect.left + target_rect.width / 2 - card_width / 2),
          viewport_width - card_width - 16,
        ),
      };
    case "right":
    default:
      return {
        top: Math.max(24, Math.min(target_rect.top + target_rect.height / 2 - card_height / 2, viewport_height - card_height - 24)),
        left: Math.min(
          target_rect.right + gutter,
          viewport_width - card_width - 16,
        ),
      };
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

  if (typeof document === "undefined" || !step) {
    return null;
  }

  const placement = step.placement ?? (step.target ? "right" : "center");
  const popover_position = get_popover_position(
    placement,
    target_rect,
    window.innerWidth,
    window.innerHeight,
    step,
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
        <div
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
  );

  return createPortal(overlay, document.body);
}

export function OnboardingTourProvider({ children }: { children: ReactNode }) {
  const tours_ref = useRef<Record<string, OnboardingTourDefinition>>({});
  const [completed_tours, set_completed_tours] = useState<Record<string, boolean>>(
    () => read_completed_tours(),
  );
  const [active_tour, set_active_tour] = useState<ActiveTourState | null>(null);

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

  const context_value = useMemo<OnboardingTourContextValue>(() => ({
    register_tour,
    unregister_tour,
    start_tour,
    close_tour,
    next_step,
    previous_step,
    has_completed_tour,
    active_tour_id: active_tour?.tour_id ?? null,
  }), [
    active_tour?.tour_id,
    close_tour,
    has_completed_tour,
    next_step,
    previous_step,
    register_tour,
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
