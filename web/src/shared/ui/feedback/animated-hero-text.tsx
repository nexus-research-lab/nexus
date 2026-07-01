"use client";

import { CSSProperties, useEffect, useRef, useState } from "react";
import { prepareWithSegments } from "@chenglou/pretext";
import { cn } from "@/lib/utils";

// ─── AnimatedHeroText ────────────────────────────────────────────────────────
// Uses pretext to split text into grapheme clusters (handles CJK + emoji + bidi)
// then reveals each grapheme with a stagger CSS transition.

interface AnimatedHeroTextProps {
  text: string;
  class_name?: string;
  /** Per-grapheme stagger interval in ms */
  stagger_ms?: number;
  /** Delay before first grapheme starts appearing */
  initial_delay_ms?: number;
}

// Intl.Segmenter is available in TypeScript ≥ 4.7 / ES2022 lib; cast via unknown
// for envs that ship an older lib but have the runtime API.
type IntlSegmenterCtor = new (
  locale?: string,
  options?: { granularity?: "grapheme" | "word" | "sentence" },
) => { segment(input: string): Iterable<{ segment: string }> };

function split_graphemes(text: string, font: string): string[] {
  try {
    const prepared = prepareWithSegments(text, font);
    return prepared.segments;
  } catch {
    if (typeof Intl !== "undefined" && "Segmenter" in Intl) {
      const SegmenterCtor = (Intl as unknown as { Segmenter: IntlSegmenterCtor }).Segmenter;
      const seg = new SegmenterCtor(undefined, { granularity: "grapheme" });
      return Array.from(seg.segment(text), (s) => s.segment);
    }
    return [...text];
  }
}

interface KeyedGrapheme {
  char: string;
  key: string;
  position: number;
}

function get_keyed_graphemes(graphemes: string[]): KeyedGrapheme[] {
  const seen_counts = new Map<string, number>();
  const keyed_graphemes: KeyedGrapheme[] = [];
  let position = 0;

  for (const char of graphemes) {
    const occurrence = seen_counts.get(char) ?? 0;
    seen_counts.set(char, occurrence + 1);
    keyed_graphemes.push({
      char,
      key: `${char}-${occurrence}`,
      position,
    });
    position += 1;
  }

  return keyed_graphemes;
}

export function AnimatedHeroText({
  text,
  class_name,
  stagger_ms = 26,
  initial_delay_ms = 100,
}: AnimatedHeroTextProps) {
  const [graphemes, setGraphemes] = useState<string[]>([]);
  const [visible, setVisible] = useState(false);
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const el = ref.current;
    const font = el
      ? window.getComputedStyle(el).font || "800 42px system-ui"
      : "800 42px system-ui";
    setGraphemes(split_graphemes(text, font));
    const t = setTimeout(() => setVisible(true), 16);
    return () => clearTimeout(t);
  }, [text]);

  if (graphemes.length === 0) {
    return (
      <span ref={ref} className={cn("opacity-0", class_name)} aria-hidden>
        {text}
      </span>
    );
  }

  return (
    <span ref={ref} className={class_name} aria-label={text}>
      {get_keyed_graphemes(graphemes).map(({ char, key, position }) => (
        <span
          key={key}
          aria-hidden
          className="inline-block"
          style={{
            // 进入动画结束后移除最终态 transform，
            // 避免标题里的每个字长期保留独立合成层。
            ...(visible ? null : {
              opacity: 0,
              transform: "translateY(8px) scale(0.94)",
            }),
            transition: "opacity 0.4s ease, transform 0.45s cubic-bezier(0.22,1,0.36,1)",
            transitionDelay: visible ? `${initial_delay_ms + position * stagger_ms}ms` : "0ms",
            whiteSpace: char === " " ? "pre" : undefined,
          }}
        >
          {char}
        </span>
      ))}
    </span>
  );
}

// ─── FadeSlideIn ─────────────────────────────────────────────────────────────
// General-purpose entrance animation for any element.
// Fades + slides up on mount, with configurable delay.

interface FadeSlideInProps {
  children: React.ReactNode;
  delay_ms?: number;
  duration_ms?: number;
  /** translateY distance to start from (px). Negative = slide down. */
  y_offset?: number;
  class_name?: string;
  style?: CSSProperties;
}

export function FadeSlideIn({
  children,
  delay_ms = 0,
  duration_ms = 420,
  y_offset = 10,
  class_name,
  style,
}: FadeSlideInProps) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setVisible(true), 16);
    return () => clearTimeout(t);
  }, []);

  return (
    <div
      className={class_name}
      style={{
        // 容器完成进入动画后不再保留 transform，
        // 这样 launcher 推荐按钮和 Hero 分组不会持续挂在独立层上。
        ...(visible ? null : {
          opacity: 0,
          transform: `translateY(${y_offset}px)`,
        }),
        transition: `opacity ${duration_ms}ms ease, transform ${duration_ms}ms cubic-bezier(0.22,1,0.36,1)`,
        transitionDelay: `${delay_ms}ms`,
        ...style,
      }}
    >
      {children}
    </div>
  );
}
