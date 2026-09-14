/**
 * INPUT: Hero 文本、进入动效参数与布局。
 * OUTPUT: 完整可访问文本和稳定字符身份的渐显、容器进入效果。
 * POS: Hero 展示动效；字符边界归 text-graphemes，动画与减少动态效果归主题 recipe。
 */
"use client";

import { type CSSProperties, useMemo } from "react";
import { splitTextGraphemes } from "@/lib/text-graphemes";
import { cn } from "@/shared/ui/class-name";

interface AnimatedHeroTextProps {
  text: string;
  className?: string;
  /** Per-grapheme stagger interval in ms */
  staggerMs?: number;
  /** Delay before first grapheme starts appearing */
  initialDelayMs?: number;
}

interface KeyedGrapheme {
  char: string;
  key: string;
  position: number;
}

function getKeyedGraphemes(graphemes: string[]): KeyedGrapheme[] {
  const seenCounts = new Map<string, number>();
  const keyedGraphemes: KeyedGrapheme[] = [];
  let position = 0;

  for (const char of graphemes) {
    const occurrence = seenCounts.get(char) ?? 0;
    seenCounts.set(char, occurrence + 1);
    keyedGraphemes.push({
      char,
      key: `${char}-${occurrence}`,
      position,
    });
    position += 1;
  }

  return keyedGraphemes;
}

export function AnimatedHeroText({
  text,
  className,
  staggerMs = 26,
  initialDelayMs = 100,
}: AnimatedHeroTextProps) {
  const graphemes = useMemo(() => getKeyedGraphemes(splitTextGraphemes(text)), [text]);

  return (
    <span className={className} aria-label={text}>
      {graphemes.map(({ char, key, position }) => (
        <span
          key={key}
          aria-hidden
          className="ui-hero-grapheme"
          style={{
            "--ui-enter-delay": `${initialDelayMs + position * staggerMs}ms`,
            whiteSpace: char === " " ? "pre" : undefined,
          } as CSSProperties}
        >
          {char}
        </span>
      ))}
    </span>
  );
}

// 通用容器只负责挂载时的淡入上移动画，延迟和时长由消费者决定。

interface FadeSlideInProps {
  children: React.ReactNode;
  delayMs?: number;
  durationMs?: number;
  /** 初始纵向偏移；正值表示从下方向上归位。 */
  yOffset?: number;
  className?: string;
  style?: CSSProperties;
}

export function FadeSlideIn({
  children,
  delayMs = 0,
  durationMs = 420,
  yOffset = 10,
  className,
  style,
}: FadeSlideInProps) {
  return (
    <div
      className={cn("ui-fade-slide-in", className)}
      style={{
        "--ui-enter-offset": `${yOffset}px`,
        "--ui-enter-duration": `${durationMs}ms`,
        "--ui-enter-delay": `${delayMs}ms`,
        ...style,
      } as CSSProperties}
    >
      {children}
    </div>
  );
}
