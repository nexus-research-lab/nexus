/**
 * INPUT: 天空变体；配色全部来自 theme-recipes.css 中 `.sidebar-panel-sky[data-sky-variant]` 定义的 --sidebar-sky-* 变量。
 * OUTPUT: 品牌栏背后的天空插画。各变体共用 480×96 画框、斜向底色、光晕与星点，再叠加各自的主角图层：
 *   night 成形云朵 · meteor 云朵加周期流星 · dawn 晨曦日晕与暖色云 · day 晴空、日光与缓慢漂移的白云（不画星点）。
 * POS: 侧栏壳层的纯装饰层，aria-hidden 且不拦截指针；渐隐、主题明暗与减弱动态效果由外层配方负责。
 */
import { useId } from "react";

import type { SidebarSkyVariant } from "@/shared/theme/sidebar-sky-preference";

// 视图盒固定 480×96，外层以 xMinYMin slice 铺满任意侧栏宽度。
const SKY_VIEW_BOX = "0 0 480 96";

// 侧栏实际渲染约为视图盒的 0.7–1 倍，星点半径整体放大，保证 1x 屏上仍能看见。
const STAR_SCALE = 1.5;

const STARS: ReadonlyArray<readonly [cx: number, cy: number, r: number, opacity: number]> = [
  [11, 8, 0.6, 0.9],
  [29, 27, 0.4, 0.55],
  [47, 13, 0.5, 0.75],
  [66, 33, 0.35, 0.45],
  [79, 6, 0.45, 0.7],
  [97, 21, 0.6, 0.85],
  [118, 9, 0.4, 0.55],
  [133, 30, 0.5, 0.65],
  [151, 15, 0.35, 0.5],
  [169, 5, 0.55, 0.8],
  [184, 26, 0.4, 0.55],
  [203, 12, 0.6, 0.85],
  [221, 34, 0.35, 0.45],
  [237, 7, 0.45, 0.7],
  [256, 22, 0.5, 0.7],
  [273, 11, 0.35, 0.5],
  [292, 31, 0.55, 0.75],
  [309, 6, 0.4, 0.6],
  [327, 19, 0.6, 0.85],
  [346, 36, 0.35, 0.45],
  [362, 10, 0.5, 0.7],
  [381, 25, 0.4, 0.55],
  [399, 8, 0.55, 0.8],
  [418, 29, 0.35, 0.5],
  [436, 14, 0.5, 0.7],
  [455, 33, 0.4, 0.5],
  [471, 9, 0.55, 0.8],
  [58, 46, 0.35, 0.4],
  [214, 48, 0.35, 0.4],
  [372, 47, 0.35, 0.4],
];

const SPARKLES: ReadonlyArray<readonly [x: number, y: number, size: number]> = [
  [88, 37, 1.8],
  [246, 41, 1.5],
  [409, 40, 1.7],
];

// 三团云：每团由几段圆弧拼出起伏轮廓，底边落在视图盒下沿之外。
const CLOUDS: ReadonlyArray<{ d: string; opacity: number }> = [
  {
    d: "M-16 98C-18 84-8 74 6 74C9 62 20 54 33 55C40 45 54 41 66 46C75 39 90 40 98 50C108 47 121 53 124 64C136 63 147 72 147 84C155 86 160 91 161 98Z",
    opacity: 1,
  },
  {
    d: "M168 98C168 88 176 80 187 80C190 71 199 65 209 66C215 58 228 56 236 62C244 57 256 60 259 69C269 69 278 76 279 86C286 88 290 93 291 98Z",
    opacity: 0.78,
  },
  {
    d: "M300 98C299 87 308 78 320 79C323 68 334 61 346 63C353 53 368 50 379 56C388 50 402 52 407 62C418 61 428 68 429 79C440 79 450 87 451 98Z",
    opacity: 0.9,
  },
];

// 流星：起点与延迟错开，由 CSS 关键帧沿 -24° 方向划过，周期内大部分时间不可见。
const METEORS: ReadonlyArray<readonly [x: number, y: number, length: number, delay: string]> = [
  [150, 6, 46, "0s"],
  [372, 2, 38, "3.4s"],
  [262, 14, 30, "6.1s"],
];

// 白天的高空薄云：两缕拉长的卷云，叠在晴空上半段，随 CSS 缓慢左右漂移。
const DAY_WISPS: ReadonlyArray<{ d: string; className: string }> = [
  {
    d: "M36 30C52 22 74 22 92 26C108 19 132 20 148 27C160 25 172 28 176 33C150 37 114 37 84 36C64 36 46 35 36 30Z",
    className: "sidebar-sky-drift sidebar-sky-drift-a",
  },
  {
    d: "M286 20C300 13 322 13 338 18C352 12 374 13 388 20C398 19 408 22 410 26C386 29 354 29 328 28C310 27 294 25 286 20Z",
    className: "sidebar-sky-drift sidebar-sky-drift-b",
  },
];

export function SidebarNightSky({ variant = "night" }: { variant?: SidebarSkyVariant }) {
  const prefix = useId().replaceAll(":", "");
  const ids = {
    sky: `${prefix}-sky`,
    glow: `${prefix}-glow`,
    cloud: `${prefix}-cloud`,
    blur: `${prefix}-blur`,
    softBlur: `${prefix}-soft-blur`,
    meteor: `${prefix}-meteor`,
    sun: `${prefix}-sun`,
    daySun: `${prefix}-day-sun`,
  };
  // 四个变体都以底部云团收尾，下沿统一羽化进侧栏；白天只把云换成实心白云并去掉星点。

  return (
    <svg
      className="sidebar-panel-sky-art"
      fill="none"
      preserveAspectRatio="xMinYMin slice"
      viewBox={SKY_VIEW_BOX}
      xmlns="http://www.w3.org/2000/svg"
    >
      <SkyDefs ids={ids} />

      <rect fill={`url(#${ids.sky})`} height="96" width="100%" />
      <rect fill={`url(#${ids.glow})`} height="96" width="100%" />

      {variant === "dawn" ? <rect fill={`url(#${ids.sun})`} height="96" width="100%" /> : null}
      {variant === "day" ? <DayLayer ids={ids} /> : <StarField dim={variant === "dawn"} />}

      {variant === "meteor" ? <MeteorLayer ids={ids} /> : null}

      {/* 白天云朵轮廓更清楚（模糊更小），夜间保持柔光。 */}
      <g className={variant === "day" ? "sidebar-sky-drift sidebar-sky-drift-clouds" : undefined} filter={`url(#${variant === "day" ? ids.softBlur : ids.blur})`}>
        {CLOUDS.map((cloud) => (
          <path d={cloud.d} fill={`url(#${ids.cloud})`} fillOpacity={cloud.opacity} key={cloud.d} />
        ))}
      </g>
    </svg>
  );
}

type SkyIds = Record<
  "sky" | "glow" | "cloud" | "blur" | "softBlur" | "meteor" | "sun" | "daySun",
  string
>;

function SkyDefs({ ids }: { ids: SkyIds }) {
  return (
    <defs>
      {/* 底色沿对角线展开并反射重复，宽侧栏下也不会出现单调的横向色带。 */}
      <linearGradient
        id={ids.sky}
        gradientUnits="userSpaceOnUse"
        spreadMethod="reflect"
        x1="0"
        x2="220"
        y1="0"
        y2="96"
      >
        <stop offset="0" style={{ stopColor: "var(--sidebar-sky-top)" }} />
        <stop offset="0.55" style={{ stopColor: "var(--sidebar-sky-mid)" }} />
        <stop offset="1" style={{ stopColor: "var(--sidebar-sky-bottom)" }} />
      </linearGradient>
      <radialGradient
        id={ids.glow}
        cx="0"
        cy="0"
        gradientTransform="translate(250 14) rotate(140) scale(150 90)"
        gradientUnits="userSpaceOnUse"
        r="1"
      >
        <stop offset="0" stopOpacity="0.42" style={{ stopColor: "var(--sidebar-sky-glow)" }} />
        <stop offset="1" stopOpacity="0" style={{ stopColor: "var(--sidebar-sky-glow)" }} />
      </radialGradient>
      <radialGradient
        id={ids.sun}
        cx="0"
        cy="0"
        gradientTransform="translate(360 100) scale(170 70)"
        gradientUnits="userSpaceOnUse"
        r="1"
      >
        <stop offset="0" stopOpacity="0.95" style={{ stopColor: "var(--sidebar-sky-sun)" }} />
        <stop offset="0.45" stopOpacity="0.4" style={{ stopColor: "var(--sidebar-sky-sun)" }} />
        <stop offset="1" stopOpacity="0" style={{ stopColor: "var(--sidebar-sky-sun)" }} />
      </radialGradient>
      <linearGradient id={ids.cloud} gradientUnits="userSpaceOnUse" x1="0" x2="460" y1="52" y2="96">
        <stop offset="0" stopOpacity="0.62" style={{ stopColor: "var(--sidebar-sky-cloud-a)" }} />
        <stop offset="0.5" stopOpacity="0.66" style={{ stopColor: "var(--sidebar-sky-cloud-b)" }} />
        <stop offset="1" stopOpacity="0.56" style={{ stopColor: "var(--sidebar-sky-cloud-c)" }} />
      </linearGradient>
      <linearGradient id={ids.meteor} x1="1" x2="0" y1="0" y2="0">
        <stop offset="0" stopOpacity="1" style={{ stopColor: "var(--sidebar-sky-star)" }} />
        <stop offset="1" stopOpacity="0" style={{ stopColor: "var(--sidebar-sky-star)" }} />
      </linearGradient>
      {/* 白天的太阳挂在右上角：暖白核心向外散成大片柔光，不画实心圆盘，避免与品牌字标抢视线。 */}
      <radialGradient
        id={ids.daySun}
        cx="0"
        cy="0"
        gradientTransform="translate(430 4) scale(150 86)"
        gradientUnits="userSpaceOnUse"
        r="1"
      >
        <stop offset="0" stopOpacity="0.95" style={{ stopColor: "var(--sidebar-sky-sun)" }} />
        <stop offset="0.18" stopOpacity="0.7" style={{ stopColor: "var(--sidebar-sky-sun)" }} />
        <stop offset="0.55" stopOpacity="0.18" style={{ stopColor: "var(--sidebar-sky-sun)" }} />
        <stop offset="1" stopOpacity="0" style={{ stopColor: "var(--sidebar-sky-sun)" }} />
      </radialGradient>
      <filter id={ids.blur} filterUnits="userSpaceOnUse" height="140" width="540" x="-30" y="-20">
        <feGaussianBlur stdDeviation="4" />
      </filter>
      <filter id={ids.softBlur} filterUnits="userSpaceOnUse" height="160" width="560" x="-40" y="-30">
        <feGaussianBlur stdDeviation="2.2" />
      </filter>
    </defs>
  );
}

function StarField({ dim }: { dim: boolean }) {
  return (
    <g
      className="sidebar-panel-sky-stars"
      opacity={dim ? 0.55 : 1}
      style={{ fill: "var(--sidebar-sky-star)" }}
    >
      {STARS.map(([cx, cy, r, opacity]) => (
        <circle cx={cx} cy={cy} fillOpacity={opacity} key={`${cx}-${cy}`} r={r * STAR_SCALE} />
      ))}
      <g strokeLinecap="round" strokeOpacity="0.75" strokeWidth="0.6" style={{ stroke: "var(--sidebar-sky-star)" }}>
        {SPARKLES.map(([x, y, size]) => (
          <path d={`M${x - size} ${y}H${x + size}M${x} ${y - size}V${y + size}`} key={`${x}-${y}`} />
        ))}
      </g>
    </g>
  );
}

function MeteorLayer({ ids }: { ids: SkyIds }) {
  return (
    <g>
      {METEORS.map(([x, y, length, delay]) => (
        <g key={`${x}-${y}`} transform={`translate(${x} ${y}) rotate(156)`}>
          <g className="sidebar-sky-meteor" style={{ animationDelay: delay }}>
            <rect fill={`url(#${ids.meteor})`} height="1" rx="0.5" width={length} x={-length} y="-0.5" />
            <circle cx="0" cy="0" r="1.1" style={{ fill: "var(--sidebar-sky-star)" }} />
          </g>
        </g>
      ))}
    </g>
  );
}

function DayLayer({ ids }: { ids: SkyIds }) {
  return (
    <g>
      <rect className="sidebar-sky-sun-breathe" fill={`url(#${ids.daySun})`} height="96" width="100%" />
      <g filter={`url(#${ids.softBlur})`} style={{ fill: "var(--sidebar-sky-wisp)" }}>
        {DAY_WISPS.map((wisp) => (
          <path className={wisp.className} d={wisp.d} key={wisp.className} />
        ))}
      </g>
    </g>
  );
}
