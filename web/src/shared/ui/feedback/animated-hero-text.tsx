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

function split_graphemes(text: string, font: string): string[] {
  try {
    const prepared = prepareWithSegments(text, font);
    return prepared.segments;
  } catch {
    if (typeof Intl !== "undefined" && "Segmenter" in Intl) {
      const seg = new (Intl as any).Segmenter(undefined, { granularity: "grapheme" });
      return [...seg.segment(text)].map((s: any) => s.segment as string);
    }
    return [...text];
  }
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
      {graphemes.map((char, i) => (
        <span
          key={i}
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
            transitionDelay: visible ? `${initial_delay_ms + i * stagger_ms}ms` : "0ms",
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

// ─── StaggerList ─────────────────────────────────────────────────────────────
// Wraps a list of items and applies staggered FadeSlideIn to each child.

interface StaggerListProps {
  children: React.ReactNode[];
  base_delay_ms?: number;
  stagger_ms?: number;
  duration_ms?: number;
  y_offset?: number;
  class_name?: string;
  item_class_name?: string;
}

export function StaggerList({
  children,
  base_delay_ms = 0,
  stagger_ms = 55,
  duration_ms = 380,
  y_offset = 8,
  class_name,
  item_class_name,
}: StaggerListProps) {
  return (
    <div className={class_name}>
      {children.map((child, i) => (
        <FadeSlideIn
          key={i}
          delay_ms={base_delay_ms + i * stagger_ms}
          duration_ms={duration_ms}
          y_offset={y_offset}
          class_name={item_class_name}
          style={{ display: "contents" }}
        >
          {child}
        </FadeSlideIn>
      ))}
    </div>
  );
}
