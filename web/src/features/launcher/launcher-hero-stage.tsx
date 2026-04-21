/**
 * =====================================================
 * @File   ：launcher-hero-stage.tsx
 * @Date   ：2026-04-16 16:22
 * @Author ：leemysw
 * 2026-04-16 16:22   Create
 * =====================================================
 */

"use client";

import { memo, useCallback, useEffect, useMemo, useRef, useState, type MouseEvent } from "react";
import { ArrowRight, ArrowUp, MessageSquare } from "lucide-react";

import { HeroBlobShell } from "@/features/launcher/launcher-glass-shell";
import { MentionTargetItem, MentionTargetPopover } from "@/features/conversation/shared/mention-popover";
import { cn } from "@/lib/utils";
import { ANIMATIONS } from "@/config/animation-assets";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";
import { AnimatedHeroText, FadeSlideIn } from "@/shared/ui/feedback/animated-hero-text";

import { AgentPile } from "./launcher-agent-pile";
import {
  find_launcher_mention_match,
  is_launcher_chip_truncated,
  truncate_launcher_chip_label,
} from "./launcher-console-helpers";
import { HeroStageProps, LauncherMentionMatch } from "./launcher-console-types";

const MemoAgentPile = memo(AgentPile);

export const LauncherHeroStage = memo(function LauncherHeroStage({
  current_agent_id,
  decorative_tokens,
  mention_targets,
  on_enter_home,
  on_open_main_agent_dm,
  on_query_change,
  on_select_agent,
  on_open_recent_entry,
  on_submit,
  query,
  recent_entries,
  is_query_loading,
}: HeroStageProps) {
  const { t } = useI18n();
  const is_composing_ref = useRef(false);
  const input_ref = useRef<HTMLInputElement>(null);
  const [local_query, set_local_query] = useState(query);
  const [mention_match, set_mention_match] = useState<LauncherMentionMatch | null>(null);

  const visible_mention_targets = useMemo(() => {
    if (!mention_match) {
      return [];
    }
    return mention_targets.filter((item) => (
      mention_match.trigger === "@"
        ? item.kind === "agent"
        : item.kind === "room"
    ));
  }, [mention_match, mention_targets]);

  const sync_mention_match = useCallback((value: string, cursor_pos: number) => {
    set_mention_match(find_launcher_mention_match(value, cursor_pos));
  }, []);

  const handle_mention_close = useCallback(() => {
    set_mention_match(null);
  }, []);

  const handle_enter_home_click = useCallback((event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    on_enter_home();
  }, [on_enter_home]);

  const handle_query_change = useCallback((value: string) => {
    set_local_query(value);
    on_query_change(value);
    const cursor_pos = input_ref.current?.selectionStart ?? value.length;
    sync_mention_match(value, cursor_pos);
  }, [on_query_change, sync_mention_match]);

  const handle_mention_select = useCallback((item: MentionTargetItem) => {
    if (!mention_match) {
      return;
    }
    const cursor_pos = input_ref.current?.selectionStart ?? local_query.length;
    const before = local_query.slice(0, mention_match.start_pos);
    const after = local_query.slice(cursor_pos);
    const next_query = `${before}${mention_match.trigger}${item.label} ${after}`;
    set_local_query(next_query);
    on_query_change(next_query);
    set_mention_match(null);

    requestAnimationFrame(() => {
      const next_cursor = mention_match.start_pos + item.label.length + 2;
      input_ref.current?.setSelectionRange(next_cursor, next_cursor);
      input_ref.current?.focus();
    });
  }, [local_query, mention_match, on_query_change]);

  useEffect(() => {
    set_local_query(query);
  }, [query]);

  useEffect(() => {
    if (!local_query) {
      set_mention_match(null);
    }
  }, [local_query]);

  const handle_submit = useCallback(() => {
    const trimmed_query = local_query.trim();
    if (!trimmed_query) {
      return;
    }

    const did_submit = on_submit(trimmed_query);
    if (!did_submit) {
      return;
    }

    // 提交后先在本地立即清空，避免受控值回流慢一拍。
    set_local_query("");
    on_query_change("");
    set_mention_match(null);
  }, [local_query, on_query_change, on_submit]);

  return (
    <div className="relative z-10 flex w-full max-w-[1180px] flex-col items-center" onClick={(e) => e.stopPropagation()}>
      <HeroBlobShell class_name="z-10 transition-transform duration-500 ease-out">
        <div className="space-y-3 sm:space-y-4">
          <FadeSlideIn delay_ms={0} duration_ms={380} y_offset={6}>
            <div className="flex flex-col items-center gap-2.5">
              <button
                className="group inline-flex items-center gap-3 rounded-full px-2 py-2 pr-4 text-left transition duration-200 ease-out hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/45"
                style={{
                  background: "color-mix(in srgb, var(--launcher-input-fill) 92%, rgba(255, 255, 255, 0.12))",
                  boxShadow: "inset 0 0 0 1px var(--launcher-input-stroke), 0 12px 26px rgba(48, 63, 88, 0.10)",
                  color: "var(--launcher-input-text)",
                }}
                onClick={handle_enter_home_click}
                type="button"
              >
                <span
                  className="inline-flex min-h-8 items-center justify-center rounded-full px-3 text-[10px] font-semibold tracking-[0.22em]"
                  style={{
                    background: "color-mix(in srgb, var(--launcher-input-inner-fill) 68%, rgba(255, 255, 255, 0.34))",
                    boxShadow: "inset 0 0 0 1px var(--launcher-input-inner-stroke)",
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
          </FadeSlideIn>
          <div className="relative inline-block">
            <LottiePlayer
              class_name="pointer-events-none absolute -right-4 -top-5 h-12 w-12 opacity-[0.46] sm:-right-16 sm:-top-14 sm:h-24 sm:w-24"
              inline_style={undefined}
              src={ANIMATIONS.SPARKLES}
            />
            <h1 className="mb-2 text-[24px] font-extrabold leading-[1.12] tracking-[-0.05em] text-foreground/96 sm:text-[42px] sm:leading-[1.05]">
              <AnimatedHeroText text={t("launcher.hero_title")} initial_delay_ms={80} stagger_ms={26} />
            </h1>
          </div>
        </div>

        <div className="mt-8 sm:mt-10">
          <FadeSlideIn delay_ms={440} duration_ms={420} y_offset={10}>
            <div
              className="mx-auto w-full max-w-[326px] rounded-2xl border px-4 py-1 sm:max-w-[420px] "
              style={{
                background: "linear-gradient(180deg, var(--launcher-input-fill), var(--launcher-input-inner-fill))",
                borderColor: "var(--launcher-input-stroke)",
                boxShadow: "inset 0 1px 0 var(--launcher-input-inner-stroke), 0 14px 30px rgba(56, 72, 98, 0.10)",
              }}
            >
              <div className="relative flex min-w-0 items-center gap-2.5 sm:gap-3">
                {mention_match ? (
                  <MentionTargetPopover
                    anchor_rect={input_ref.current?.getBoundingClientRect() ?? null}
                    filter={mention_match.filter}
                    items={visible_mention_targets}
                    on_close={handle_mention_close}
                    on_select={handle_mention_select}
                    placement="below"
                  />
                ) : null}
                <MessageSquare className="h-4.5 w-4.5" style={{ color: "var(--launcher-input-icon)" }} />
                <input
                  ref={input_ref}
                  className="flex-1 bg-transparent text-[14px] outline-none shadow-none ring-0 placeholder:text-(--launcher-input-placeholder) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none sm:text-[15px]"
                  style={{ color: "var(--launcher-input-text)" }}
                  onBlur={() => {
                    requestAnimationFrame(() => {
                      if (document.activeElement !== input_ref.current) {
                        set_mention_match(null);
                      }
                    });
                  }}
                  onChange={(event) => handle_query_change(event.target.value)}
                  onCompositionEnd={() => {
                    is_composing_ref.current = false;
                  }}
                  onCompositionStart={() => {
                    is_composing_ref.current = true;
                  }}
                  onKeyDown={(event) => {
                    if (is_composing_ref.current || event.nativeEvent.isComposing) {
                      return;
                    }
                    if (mention_match && visible_mention_targets.length > 0 && ["ArrowDown", "ArrowUp", "Enter", "Tab", "Escape"].includes(event.key)) {
                      return;
                    }
                    if (event.key === "Enter") {
                      event.preventDefault();
                      handle_submit();
                    }
                  }}
                  onSelect={(event) => {
                    const target = event.target as HTMLInputElement;
                    sync_mention_match(target.value, target.selectionStart ?? target.value.length);
                  }}
                  value={local_query}
                  placeholder={t("launcher.query_placeholder")}
                  disabled={is_query_loading}
                />
                <button
                  className={cn(
                    "inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full border transition duration-150 ease-out hover:-translate-y-0.5 sm:h-11 sm:w-11",
                    is_query_loading && "cursor-not-allowed opacity-(--disabled-opacity) hover:translate-y-0",
                  )}
                  style={{
                    background: "var(--launcher-submit-background)",
                    borderColor: "rgba(255,255,255,0.34)",
                    boxShadow: "inset 0 1px 0 rgba(255,255,255,0.26), var(--launcher-submit-shadow)",
                    color: "var(--launcher-submit-color)",
                  }}
                  onClick={handle_submit}
                  type="button"
                  disabled={is_query_loading}
                >
                  {is_query_loading ? (
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-(--divider-strong-color) border-t-transparent" />
                  ) : (
                    <ArrowUp className="h-4 w-4" />
                  )}
                </button>
              </div>
            </div>
          </FadeSlideIn>

          <div className={cn(
            "flex flex-wrap items-center justify-center gap-2",
            "mt-3 sm:mt-4",
          )}>
            {recent_entries.map((entry, index) => (
              <FadeSlideIn key={entry.key} delay_ms={580 + index * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
                <div className="group relative inline-flex">
                  {is_launcher_chip_truncated(entry.label) ? (
                    <div
                      className="pointer-events-none absolute left-1/2 top-full z-20 mt-2 w-max max-w-[220px] -translate-x-1/2 translate-y-1 rounded-2xl px-3 py-2 text-center text-xs font-medium leading-5 opacity-0 shadow-[0_18px_42px_rgba(38,52,76,0.16)] transition duration-200 ease-out group-hover:translate-y-0 group-hover:opacity-100 group-focus-within:translate-y-0 group-focus-within:opacity-100"
                      style={{
                        background: "rgba(247, 249, 253, 0.96)",
                        boxShadow: "0 18px 42px rgba(38, 52, 76, 0.16), inset 0 0 0 1px rgba(255, 255, 255, 0.52)",
                        color: "rgba(39, 50, 74, 0.88)",
                      }}
                    >
                      {entry.type === "room" ? "#" : ""}
                      {entry.label}
                    </div>
                  ) : null}
                  <button
                    aria-label={entry.type === "room" ? `房间 ${entry.label}` : `私聊 ${entry.label}`}
                    className="inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-medium transition duration-150 ease-out hover:-translate-y-0.5 sm:text-sm"
                    style={{
                      background: entry.type === "room"
                        ? "var(--launcher-room-chip-background)"
                        : "var(--launcher-agent-chip-background)",
                      boxShadow: entry.type === "room"
                        ? "inset 0 0 0 1px var(--launcher-room-chip-border)"
                        : "inset 0 0 0 1px var(--launcher-agent-chip-border)",
                      color: entry.type === "room"
                        ? "var(--launcher-room-chip-text)"
                        : "var(--launcher-agent-chip-text)",
                    }}
                    onClick={(event) => {
                      event.stopPropagation();
                      on_open_recent_entry(entry);
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
                    {truncate_launcher_chip_label(entry.label)}
                  </button>
                </div>
              </FadeSlideIn>
            ))}

            <FadeSlideIn delay_ms={580 + recent_entries.length * 55} duration_ms={360} y_offset={6} style={{ display: "inline-flex" }}>
              <button
                className="px-2 text-xs font-medium transition-colors duration-150 ease-out hover:text-(--launcher-handoff-hover-color) sm:text-sm"
                style={{ color: "var(--launcher-handoff-color)" }}
                onClick={() => on_open_main_agent_dm(query)}
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
        class_name="hidden min-[400px]:block"
        current_agent_id={current_agent_id}
        on_select_agent={on_select_agent}
        tokens={decorative_tokens}
      />
    </div>
  );
});
