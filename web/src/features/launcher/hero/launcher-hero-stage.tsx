"use client";

import {
  memo,
  useCallback,
  type MouseEvent,
} from "react";
import { ArrowRight, MessageSquare } from "lucide-react";

import { LAUNCHER_TOUR_ANCHORS } from "@/features/onboarding/tours/launcher-tour";
import { cn } from "@/shared/ui/class-name";
import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import {
  AnimatedHeroText,
  FadeSlideIn,
} from "@/shared/ui/feedback/animated-hero-text";
import { MentionTargetPopover } from "@/shared/ui/mention/mention-target-popover";

import type { HeroStageProps } from "../console/launcher-console-types";
import {
  isLauncherChipTruncated,
  truncateLauncherChipLabel,
} from "../console/launcher-console-helpers";
import { HeroBlobShell } from "./launcher-glass-shell";
import { AgentPile } from "./pile/launcher-agent-pile";
import { useLauncherQueryInput } from "./use-launcher-query-input";

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
    <div
      className="relative z-10 flex w-full max-w-[1180px] flex-col items-center"
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
      role="presentation"
    >
      <HeroBlobShell className="z-10 transition-transform duration-500 ease-out">
        <div className="space-y-3 sm:space-y-4">
          <FadeSlideIn delayMs={0} durationMs={380} yOffset={6}>
            <div className="flex flex-col items-center gap-2.5">
              <div className="flex items-center gap-2">
                <button
                  data-tour-anchor={LAUNCHER_TOUR_ANCHORS.enter_app}
                  className="group inline-flex items-center gap-3 rounded-full px-2 py-2 pr-4 text-left transition duration-200 ease-out hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/45"
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
                    className="inline-flex min-h-8 items-center justify-center rounded-full px-3 text-[10px] font-semibold tracking-[0.22em]"
                    style={{
                      background:
                        "color-mix(in srgb, var(--launcher-input-inner-fill) 68%, rgba(255, 255, 255, 0.34))",
                      boxShadow:
                        "inset 0 0 0 1px var(--launcher-input-inner-stroke)",
                    }}
                  >
                    APP
                  </span>
                  <span className="text-[12px] font-semibold tracking-[0.12em] text-foreground/90 sm:text-[13px]">
                    {t("launcher.enter_app")}
                  </span>
                  <ArrowRight className="h-3.5 w-3.5 transition-transform duration-200 ease-out group-hover:translate-x-0.5" />
                </button>

              </div>
            </div>
          </FadeSlideIn>
          <div className="relative inline-block">
            <LottiePlayer
              className="pointer-events-none absolute -right-4 -top-5 h-12 w-12 opacity-[0.46] sm:-right-16 sm:-top-14 sm:h-24 sm:w-24"
              inlineStyle={undefined}
              src={ANIMATIONS.SPARKLES}
            />
            <h1 className="mb-2 text-[24px] font-extrabold leading-[1.12] tracking-[-0.05em] text-foreground/96 sm:text-[42px] sm:leading-[1.05]">
              <AnimatedHeroText
                text={t("launcher.hero_title")}
                initialDelayMs={80}
                staggerMs={26}
              />
            </h1>
          </div>
        </div>

        <div className="mt-8 sm:mt-10">
          <FadeSlideIn delayMs={440} durationMs={420} yOffset={10}>
            <div
              data-tour-anchor={LAUNCHER_TOUR_ANCHORS.composer}
              className="mx-auto w-full max-w-[326px] rounded-2xl border px-4 py-1 sm:max-w-[420px] "
              style={{
                background:
                  "linear-gradient(180deg, var(--launcher-input-fill), var(--launcher-input-inner-fill))",
                borderColor: "var(--launcher-input-stroke)",
                boxShadow:
                  "inset 0 1px 0 var(--launcher-input-inner-stroke), 0 14px 30px rgba(56, 72, 98, 0.10)",
              }}
            >
              <div className="relative flex min-w-0 items-center gap-2.5 sm:gap-3">
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
                  aria-label="输入启动器指令"
                  ref={queryInput.input.ref}
                  className="flex-1 bg-transparent text-[14px] outline-none shadow-none ring-0 placeholder:text-(--launcher-input-placeholder) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none sm:text-[15px]"
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
                    "inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full border transition duration-150 ease-out hover:-translate-y-0.5 sm:h-11 sm:w-11",
                    isQueryLoading &&
                      "cursor-not-allowed opacity-(--disabled-opacity) hover:translate-y-0",
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
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-(--divider-strong-color) border-t-transparent" />
                  ) : (
                    <img alt="Send" className="h-10 w-10 object-contain sm:h-11 sm:w-11" src="/nexus/launcher-send-mascot.png" />
                  )}
                </button>
              </div>
            </div>
          </FadeSlideIn>

          <div
            data-tour-anchor={LAUNCHER_TOUR_ANCHORS.recent}
            className={cn(
              "mx-auto flex w-full max-w-[326px] flex-wrap items-center justify-center gap-1 sm:max-w-[420px]",
              "mt-3 sm:mt-4",
            )}
          >
            {recentEntries.map((entry, index) => (
              <FadeSlideIn
                key={entry.key}
                delayMs={580 + index * 55}
                durationMs={360}
                yOffset={6}
                style={{ display: "inline-flex" }}
              >
                <div className="group relative inline-flex">
                  {isLauncherChipTruncated(entry.label) ? (
                    <div
                      className="pointer-events-none absolute left-1/2 top-full z-20 mt-2 w-max max-w-[220px] -translate-x-1/2 translate-y-1 rounded-2xl px-3 py-2 text-center text-xs font-medium leading-5 opacity-0 shadow-[0_18px_42px_rgba(38,52,76,0.16)] transition duration-200 ease-out group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:opacity-100"
                      style={{
                        background: "rgba(247, 249, 253, 0.96)",
                        boxShadow:
                          "0 18px 42px rgba(38, 52, 76, 0.16), inset 0 0 0 1px rgba(255, 255, 255, 0.52)",
                        color: "rgba(39, 50, 74, 0.88)",
                      }}
                    >
                      {entry.type === "room" ? "#" : ""}
                      {entry.label}
                    </div>
                  ) : null}
                  <button
                    aria-label={
                      entry.type === "room"
                        ? `房间 ${entry.label}`
                        : `私聊 ${entry.label}`
                    }
                    className="inline-flex items-center gap-1 rounded-full px-2 py-1.5 text-xs font-medium transition duration-150 ease-out hover:-translate-y-0.5 sm:text-sm"
                    style={{
                      background:
                        entry.type === "room"
                          ? "var(--launcher-room-chip-background)"
                          : "var(--launcher-agent-chip-background)",
                      boxShadow:
                        entry.type === "room"
                          ? "inset 0 0 0 1px var(--launcher-room-chip-border)"
                          : "inset 0 0 0 1px var(--launcher-agent-chip-border)",
                      color:
                        entry.type === "room"
                          ? "var(--launcher-room-chip-text)"
                          : "var(--launcher-agent-chip-text)",
                    }}
                    onClick={(event) => {
                      event.stopPropagation();
                      onOpenRecentEntry(entry);
                    }}
                    type="button"
                  >
                    {entry.type === "dm" ? (
                      <span
                        className="h-4 w-4 rounded-full"
                        style={{
                          backgroundColor: index === 0 ? "#bff0ca" : "#ffd7b8",
                          border: `1px solid ${index === 0 ? "#7fe3a8" : "#e3c6ad"}`,
                        }}
                      />
                    ) : null}
                    {entry.type === "room" ? "#" : ""}
                    {truncateLauncherChipLabel(entry.label)}
                  </button>
                </div>
              </FadeSlideIn>
            ))}

            <FadeSlideIn
              delayMs={580 + recentEntries.length * 55}
              durationMs={360}
              yOffset={6}
              style={{ display: "inline-flex" }}
            >
              <button
                data-tour-anchor={LAUNCHER_TOUR_ANCHORS.handoff}
                className="px-1 text-xs font-medium transition-colors duration-150 ease-out hover:text-(--launcher-handoff-hover-color) sm:text-sm"
                style={{ color: "var(--launcher-handoff-color)" }}
                onClick={() => onOpenMainAgentDm(queryInput.input.value)}
                type="button"
              >
                <span className="inline-flex items-center gap-1.5">
                  {t("launcher.handoff")}
                  <ArrowRight className="h-3.5 w-3.5" />
                </span>
              </button>
            </FadeSlideIn>
          </div>
        </div>
      </HeroBlobShell>

      <MemoAgentPile
        className="hidden min-[400px]:block"
        currentAgentId={currentAgentId}
        onSelectAgent={onSelectAgent}
        tokens={decorativeTokens}
      />
    </div>
  );
});
