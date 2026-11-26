/**
 * INPUT: 无；配色全部来自 theme-recipes.css 中 `.sidebar-panel-sky` 定义的 --sidebar-sky-* 变量。
 * OUTPUT: 品牌栏背后的夜空插画：斜向夜色、右上光晕、星点与十字闪光，以及压在下沿的成形云朵。
 * POS: 侧栏壳层的纯装饰层，aria-hidden 且不拦截指针；渐隐与主题适配由外层配方负责。
 */
import { useId } from "react";

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

export function SidebarNightSky() {
  const prefix = useId().replaceAll(":", "");
  const skyId = `${prefix}-sky`;
  const glowId = `${prefix}-glow`;
  const cloudId = `${prefix}-cloud`;
  const blurId = `${prefix}-blur`;

  return (
    <svg
      className="sidebar-panel-sky-art"
      fill="none"
      preserveAspectRatio="xMinYMin slice"
      viewBox={SKY_VIEW_BOX}
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        {/* 夜色沿对角线展开并反射重复，宽侧栏下也不会出现单调的横向色带。 */}
        <linearGradient
          id={skyId}
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
          id={glowId}
          cx="0"
          cy="0"
          gradientTransform="translate(250 14) rotate(140) scale(150 90)"
          gradientUnits="userSpaceOnUse"
          r="1"
        >
          <stop offset="0" stopOpacity="0.42" style={{ stopColor: "var(--sidebar-sky-glow)" }} />
          <stop offset="1" stopOpacity="0" style={{ stopColor: "var(--sidebar-sky-glow)" }} />
        </radialGradient>
        <linearGradient id={cloudId} gradientUnits="userSpaceOnUse" x1="0" x2="460" y1="52" y2="96">
          <stop offset="0" stopOpacity="0.62" style={{ stopColor: "var(--sidebar-sky-cloud-a)" }} />
          <stop offset="0.5" stopOpacity="0.66" style={{ stopColor: "var(--sidebar-sky-cloud-b)" }} />
          <stop offset="1" stopOpacity="0.56" style={{ stopColor: "var(--sidebar-sky-cloud-c)" }} />
        </linearGradient>
        <filter id={blurId} filterUnits="userSpaceOnUse" height="140" width="540" x="-30" y="-20">
          <feGaussianBlur stdDeviation="4" />
        </filter>
      </defs>

      <rect fill={`url(#${skyId})`} height="96" width="100%" />
      <rect fill={`url(#${glowId})`} height="96" width="100%" />

      <g className="sidebar-panel-sky-stars" style={{ fill: "var(--sidebar-sky-star)" }}>
        {STARS.map(([cx, cy, r, opacity]) => (
          <circle cx={cx} cy={cy} fillOpacity={opacity} key={`${cx}-${cy}`} r={r * STAR_SCALE} />
        ))}
        <g strokeLinecap="round" strokeOpacity="0.75" strokeWidth="0.6" style={{ stroke: "var(--sidebar-sky-star)" }}>
          {SPARKLES.map(([x, y, size]) => (
            <path d={`M${x - size} ${y}H${x + size}M${x} ${y - size}V${y + size}`} key={`${x}-${y}`} />
          ))}
        </g>
      </g>

      <g filter={`url(#${blurId})`}>
        {CLOUDS.map((cloud) => (
          <path d={cloud.d} fill={`url(#${cloudId})`} fillOpacity={cloud.opacity} key={cloud.d} />
        ))}
      </g>
    </svg>
  );
}
