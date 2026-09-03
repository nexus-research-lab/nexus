"use client";

import {
  memo,
  useCallback,
  type MouseEvent,
} from "react";
import { ArrowRight, LoaderCircle, MessageSquare } from "lucide-react";

import { LAUNCHER_TOUR_ANCHORS } from "@/features/onboarding/tours/launcher-tour";
import { cn } from "@/shared/ui/class-name";
import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  AnimatedHeroText,
  FadeSlideIn,
} from "@/shared/ui/feedback/animated-hero-text";
import { MentionTargetPopover } from "@/shared/ui/mention/mention-target-popover";

import type { HeroStageProps } from "../console/launcher-console-types";
import { HeroBlobShell } from "./launcher-glass-shell";
import { LauncherRecentEntries } from "./launcher-recent-entries";
import { AgentPile } from "./pile/launcher-agent-pile";
import { useLauncherQueryInput } from "./use-launcher-query-input";
import { useLauncherStageScale } from "./use-launcher-stage-scale";

const MemoAgentPile = memo(AgentPile);

export const LauncherHeroStage = memo(function LauncherHeroStage({
  currentAgentId,
  decorativeTokens,
  mentionTargets,
  onEnterHome,
  onOpenMainAgentDm,
  onQueryChange,
  onSelectAgent,
  onOpenRecentEntry,
  onSubmit,
  query,
  recentEntries,
  isQueryLoading,
}: HeroStageProps) {
  const { t } = useI18n();
  const stage = useLauncherStageScale();
  const queryInput = useLauncherQueryInput({
    mentionTargets,
    onQueryChange,
    onSubmit,
    query,
  });

  const handleEnterHomeClick = useCallback(
    (event: MouseEvent<HTMLButtonElement>) => {
      event.stopPropagation();
      onEnterHome();
    },
    [onEnterHome],
  );

  return (
    <div className="relative z-10 h-full w-full" ref={stage.viewportRef}>
      <div
        className="absolute left-1/2 top-[30%] w-[980px]"
        style={{
          transform: `translate(-50%, -50%) scale(${stage.scale})`,
        }}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => e.stopPropagation()}
        role="presentation"
      >
      <HeroBlobShell className="z-10 transition-transform duration-500 ease-out">
        <div className="space-y-4">
          <FadeSlideIn delayMs={0} durationMs={380} yOffset={6}>
            <div className="flex flex-col items-center gap-2.5">
              <div className="flex items-center gap-2">
                <button
                  data-tour-anchor={LAUNCHER_TOUR_ANCHORS.enter_app}
                  className="group inline-flex items-center gap-3 rounded-full px-2 py-2 pr-4 text-left transition-colors duration-200 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/45"
                  style={{
                    background:
                      "color-mix(in srgb, var(--launcher-input-fill) 92%, rgba(255, 255, 255, 0.12))",
                    boxShadow:
                      "inset 0 0 0 1px var(--launcher-input-stroke), 0 12px 26px rgba(48, 63, 88, 0.10)",
                    color: "var(--launcher-input-text)",
                  }}
                  onClick={handleEnterHomeClick}
                  type="button"
                >
                  <span
                    className="inline-flex min-h-8 items-center justify-center rounded-full px-3 text-2xs font-semibold tracking-[0.22em]"
                    style={{
                      background:
                        "color-mix(in srgb, var(--launcher-input-inner-fill) 68%, rgba(255, 255, 255, 0.34))",
                      boxShadow:
                        "inset 0 0 0 1px var(--launcher-input-inner-stroke)",
                    }}
                  >
                    APP
                  </span>
                  <span className="text-sm font-semibold tracking-[0.12em] text-foreground/90">
                    {t("launcher.enter_app")}
                  </span>
                  <ArrowRight className="h-3.5 w-3.5 transition-transform duration-200 ease-out group-hover:translate-x-0.5" />
                </button>

              </div>
            </div>
          </FadeSlideIn>
          <div className="relative inline-block">
            <LottiePlayer
              className="pointer-events-none absolute -right-16 -top-14 h-24 w-24 opacity-[0.46]"
              inlineStyle={undefined}
              src={ANIMATIONS.SPARKLES}
            />
            <h1 className="mb-2 text-2xl font-semibold leading-[1.05] tracking-[-0.02em] text-foreground/96">
              <AnimatedHeroText
                text={t("launcher.hero_title")}
                initialDelayMs={80}
                staggerMs={26}
              />
            </h1>
          </div>
        </div>

        <div className="mt-10">
          <FadeSlideIn delayMs={440} durationMs={420} yOffset={10}>
            <div
              data-tour-anchor={LAUNCHER_TOUR_ANCHORS.composer}
              className="mx-auto w-full max-w-[420px] surface-radius-lg border px-4 py-1"
              style={{
                background:
                  "linear-gradient(180deg, var(--launcher-input-fill), var(--launcher-input-inner-fill))",
                borderColor: "var(--launcher-input-stroke)",
                boxShadow:
                  "inset 0 1px 0 var(--launcher-input-inner-stroke), 0 14px 30px rgba(56, 72, 98, 0.10)",
              }}
            >
              <div className="relative flex min-w-0 items-center gap-3">
                {queryInput.mention.match ? (
                  <MentionTargetPopover
                    anchorRect={
                      queryInput.input.ref.current?.getBoundingClientRect() ?? null
                    }
                    filter={queryInput.mention.match.filter}
                    items={queryInput.mention.targets}
                    onClose={queryInput.mention.close}
                    onSelect={queryInput.mention.select}
                    placement="below"
                  />
                ) : null}
                <MessageSquare
                  className="h-4.5 w-4.5"
                  style={{ color: "var(--launcher-input-icon)" }}
                />
                <input
                  aria-label={t("launcher.query_input")}
                  ref={queryInput.input.ref}
                  className="flex-1 bg-transparent text-base outline-none shadow-none ring-0 placeholder:text-(--launcher-input-placeholder) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none"
                  style={{ color: "var(--launcher-input-text)" }}
                  onBlur={queryInput.input.onBlur}
                  onChange={queryInput.input.onChange}
                  onCompositionEnd={queryInput.input.onCompositionEnd}
                  onCompositionStart={queryInput.input.onCompositionStart}
                  onKeyDown={queryInput.input.onKeyDown}
                  onSelect={queryInput.input.onSelect}
                  value={queryInput.input.value}
                  placeholder={t("launcher.query_placeholder")}
                  disabled={isQueryLoading}
                />
                <button
                  className={cn(
                    "inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-full border transition-[background,border-color,color,opacity] duration-150 ease-out",
                    isQueryLoading && "cursor-not-allowed opacity-(--disabled-opacity)",
                  )}
                  style={{
                    background: isQueryLoading
                      ? "var(--launcher-submit-background)"
                      : "transparent",
                    borderColor: isQueryLoading
                      ? "rgba(255,255,255,0.34)"
                      : "transparent",
                    boxShadow:
                      isQueryLoading
                        ? "inset 0 1px 0 rgba(255,255,255,0.26), var(--launcher-submit-shadow)"
                        : "none",
                    color: "var(--launcher-submit-color)",
                  }}
                  onClick={queryInput.submit}
                  type="button"
                  disabled={isQueryLoading}
                >
                  {isQueryLoading ? (
                    <LoaderCircle
                      aria-hidden="true"
                      className={getUiSpinnerClassName({ size: "md" })}
                    />
                  ) : (
                    <img
                      alt={t("launcher.send")}
                      className="h-11 w-11 object-contain"
                      src="/nexus/launcher-send-mascot.png"
                    />
                  )}
                </button>
              </div>
            </div>
          </FadeSlideIn>

          <LauncherRecentEntries
            handoffLabel={t("launcher.handoff")}
            initialPrompt={queryInput.input.value}
            onHandoff={onOpenMainAgentDm}
            onOpen={onOpenRecentEntry}
            recentEntries={recentEntries}
          />
        </div>
      </HeroBlobShell>

      </div>
      <div
        className="pointer-events-none absolute bottom-0 left-1/2 origin-bottom"
        style={{ transform: `translateX(-50%) scale(${stage.scale})` }}
      >
        <MemoAgentPile
          currentAgentId={currentAgentId}
          onSelectAgent={onSelectAgent}
          tokens={decorativeTokens}
        />
      </div>
    </div>
  );
});
