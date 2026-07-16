/**
 * theme-background-pattern.ts
 *
 * 生成 SVG 平铺背景纹理，通过 CSS custom property 注入 body。
 * 统一使用等边三角形 isometric grid 骨架，仅通过色调/透明度区分主题：
 *   - light：白色描边（embossed）—— 明亮、通透
 *   - dark ：深色描边（engraved）—— 深邃、精密
 *   - rain ：深色轻描边（etched）—— 内敛、沉静
 *
 * 统一几何骨架 + 纯色 alpha 变体，
 * 线条保持低对比，但要在纯色底上可感知，不抢正文阅读层级。
 */

type BackgroundTheme = "light" | "dark" | "sunny" | "rain";
type PatternVariant = "light" | "dark" | "rain";

/* 等边三角形网格参数 — 边长 80px，tile 160 × 138.56 */
const SIDE = 80;
const TILE_W = SIDE * 2;
const TILE_H = +(SIDE * Math.sqrt(3)).toFixed(2);
const MID_Y = +(TILE_H / 2).toFixed(2);

/* ------------------------------------------------------------------ */
/*  共享骨架 — isometric triangle grid                                */
/*  6 条线段平铺后形成完整等边三角形镶嵌：                            */
/*       \/\/\/\/\/\/                                                  */
/*       /\/\/\/\/\/\                                                  */
/* ------------------------------------------------------------------ */

function buildGridSvg(strokeColor: string, strokeWidth: number): string {
  const d = [
    // 水平线
    `M0,0L${TILE_W},0`,
    `M0,${MID_Y}L${TILE_W},${MID_Y}`,
    // 右下斜线（60°）
    `M0,0L${SIDE},${TILE_H}`,
    `M${SIDE},0L${TILE_W},${TILE_H}`,
    // 左下斜线（120°）
    `M${SIDE},0L0,${TILE_H}`,
    `M${TILE_W},0L${SIDE},${TILE_H}`,
  ].join(" ");

  return [
    `<svg xmlns='http://www.w3.org/2000/svg' width='${TILE_W}' height='${TILE_H}' viewBox='0 0 ${TILE_W} ${TILE_H}'>`,
    `<path d='${d}' fill='none' stroke='${strokeColor}' stroke-width='${strokeWidth}'/>`,
    `</svg>`,
  ].join("");
}

/* Light — 白色高光叠加极淡冷灰底线，形成克制的 embossed 浮雕边缘。 */
function buildLightSvg(): string {
  return buildGridSvg("rgba(148,163,184,0.068)", 0.6);
}

/* Dark — 深色 engraved 描边，在 #131316 上形成微弱阴刻质感 */
function buildDarkSvg(): string {
  return buildGridSvg("rgba(0,0,0,0.24)", 0.6);
}

/* Rain — 极轻蚀刻，避免与雨滴 canvas overlay 冲突 */
function buildRainSvg(): string {
  return buildGridSvg("rgba(0,0,0,0.12)", 0.6);
}

/* ------------------------------------------------------------------ */
/*  Tile sizes (per variant)                                          */
/* ------------------------------------------------------------------ */

const TILE_SIZES: Record<PatternVariant, { w: number; h: number }> = {
  light: { w: TILE_W, h: TILE_H },
  dark: { w: TILE_W, h: TILE_H },
  rain: { w: TILE_W, h: TILE_H },
};

/* ------------------------------------------------------------------ */
/*  Public API                                                        */
/* ------------------------------------------------------------------ */

function resolveVariant(theme: BackgroundTheme): PatternVariant {
  if (theme === "dark") return "dark";
  if (theme === "rain") return "rain";
  return "light";
}

const BACKGROUNDS: Record<PatternVariant, string> = {
  light: "#fcfdfc",
  dark: "#131316",
  rain: "#39424d",
};

const BUILDERS: Record<PatternVariant, () => string> = {
  light: buildLightSvg,
  dark: buildDarkSvg,
  rain: buildRainSvg,
};

// memoize encoded SVGs — they never change at runtime
const cache = new Map<PatternVariant, string>();

function buildPatternUrl(variant: PatternVariant): string {
  let url = cache.get(variant);
  if (!url) {
    const svg = BUILDERS[variant]();
    url = `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
    cache.set(variant, url);
  }
  return url;
}

function applyForTheme(root: HTMLElement, theme: BackgroundTheme) {
  const variant = resolveVariant(theme);
  const size = TILE_SIZES[variant];
  const patternSize = `${size.w}px ${size.h}px`;

  root.style.setProperty("--nexus-page-pattern-size", patternSize);
  root.style.setProperty("--ambient-page-pattern-size", patternSize);

  root.style.setProperty("--nexus-page-background-light", BACKGROUNDS.light);
  root.style.setProperty("--nexus-page-background-dark", BACKGROUNDS.dark);
  root.style.setProperty("--nexus-page-background-rain", BACKGROUNDS.rain);

  root.style.setProperty("--nexus-page-pattern-light", buildPatternUrl("light"));
  root.style.setProperty("--nexus-page-pattern-dark", buildPatternUrl("dark"));
  root.style.setProperty("--nexus-page-pattern-rain", buildPatternUrl("rain"));

  root.style.setProperty("--ambient-page-pattern", buildPatternUrl(variant));
}

export function applyThemeBackgroundPattern(
  theme: BackgroundTheme,
  root: HTMLElement = document.documentElement,
) {
  applyForTheme(root, theme);
}
