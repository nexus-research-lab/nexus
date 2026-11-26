/**
 * INPUT: 天空变体；配色全部来自 theme-recipes.css 中 `.sidebar-panel-sky[data-sky-variant]` 定义的 --sidebar-sky-* 变量。
 * OUTPUT: 品牌栏背后的天空插画。各变体共用 480×96 画框、斜向底色、光晕与星点，再叠加各自的主角图层：
 *   night 成形云朵 · meteor 云朵加周期流星 · aurora 摇曳极光带 · dawn 晨曦日晕与暖色云 · constellation 星座连线与沿边流动的信号。
 * POS: 侧栏壳层的纯装饰层，aria-hidden 且不拦截指针；渐隐、主题明暗与减弱动态效果由外层配方负责。
 */
import { useId } from "react";

export type SidebarSkyVariant = "night" | "meteor" | "aurora" | "dawn" | "constellation";

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

// 极光：三条上沿起伏、向下渐隐的光带，各自以不同周期左右摆动。
const AURORA_BANDS: ReadonlyArray<{ d: string; className: string }> = [
  {
    d: "M-20 50C26 14 64 44 108 22C150 0 188 40 236 18C280-2 316 36 362 14C404-6 446 26 500 8V72H-20Z",
    className: "sidebar-sky-aurora-band sidebar-sky-aurora-band-a",
  },
  {
    d: "M-20 62C34 34 84 64 138 40C188 18 230 56 284 32C334 10 382 50 430 28C458 16 480 24 500 20V82H-20Z",
    className: "sidebar-sky-aurora-band sidebar-sky-aurora-band-b",
  },
  {
    d: "M-20 34C24 24 60 30 96 20C136 9 168 22 204 14C236 7 262 16 296 10V54C262 58 236 50 204 56C168 62 136 50 96 58C60 64 24 56 -20 62Z",
    className: "sidebar-sky-aurora-band sidebar-sky-aurora-band-c",
  },
];

// 星座：节点与边构成一张稀疏的协作网络，呼应 Nexus 的多 Agent 连接。
const CONSTELLATION_NODES: ReadonlyArray<readonly [x: number, y: number, r: number]> = [
  [22, 30, 1.3],
  [58, 14, 1],
  [92, 34, 1.6],
  [130, 18, 1.1],
  [168, 40, 1.3],
  [204, 16, 1.8],
  [242, 32, 1.1],
  [280, 12, 1.3],
  [318, 36, 1.6],
  [356, 20, 1.1],
  [394, 42, 1.3],
  [430, 16, 1.7],
  [466, 34, 1.1],
];

const CONSTELLATION_EDGES: ReadonlyArray<readonly [from: number, to: number]> = [
  [0, 1],
  [1, 2],
  [0, 2],
  [2, 3],
  [3, 5],
  [2, 4],
  [4, 5],
  [5, 6],
  [6, 7],
  [7, 8],
  [6, 8],
  [8, 9],
  [9, 11],
  [8, 10],
  [10, 11],
  [11, 12],
];

// 沿网络流动的两段信号路径，以虚线偏移动画表达“消息在 Agent 之间传递”。
const CONSTELLATION_SIGNALS: ReadonlyArray<readonly number[]> = [
  [0, 2, 4, 5, 6, 8, 10, 11, 12],
  [1, 3, 5, 7, 8, 9, 11],
];

const PULSING_NODES = new Set([2, 5, 8, 11]);

export function SidebarNightSky({ variant = "night" }: { variant?: SidebarSkyVariant }) {
  const prefix = useId().replaceAll(":", "");
  const ids = {
    sky: `${prefix}-sky`,
    glow: `${prefix}-glow`,
    cloud: `${prefix}-cloud`,
    blur: `${prefix}-blur`,
    softBlur: `${prefix}-soft-blur`,
    meteor: `${prefix}-meteor`,
    aurora: `${prefix}-aurora`,
    rays: `${prefix}-rays`,
    sun: `${prefix}-sun`,
  };
  const showClouds = variant === "night" || variant === "meteor" || variant === "dawn";

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
      {variant === "aurora" ? <AuroraLayer ids={ids} /> : null}

      <StarField dim={variant === "dawn" || variant === "aurora"} />

      {variant === "meteor" ? <MeteorLayer ids={ids} /> : null}
      {variant === "constellation" ? <ConstellationLayer /> : null}

      {showClouds ? (
        <g filter={`url(#${ids.blur})`}>
          {CLOUDS.map((cloud) => (
            <path d={cloud.d} fill={`url(#${ids.cloud})`} fillOpacity={cloud.opacity} key={cloud.d} />
          ))}
        </g>
      ) : null}
    </svg>
  );
}

type SkyIds = Record<
  "sky" | "glow" | "cloud" | "blur" | "softBlur" | "meteor" | "aurora" | "rays" | "sun",
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
      <linearGradient id={ids.aurora} x1="0" x2="0" y1="0" y2="1">
        <stop offset="0" stopOpacity="0" style={{ stopColor: "var(--sidebar-sky-aurora-a)" }} />
        <stop offset="0.18" stopOpacity="0.9" style={{ stopColor: "var(--sidebar-sky-aurora-a)" }} />
        <stop offset="0.6" stopOpacity="0.45" style={{ stopColor: "var(--sidebar-sky-aurora-b)" }} />
        <stop offset="1" stopOpacity="0" style={{ stopColor: "var(--sidebar-sky-aurora-c)" }} />
      </linearGradient>
      {/* 极光的竖向光帘：不等宽、不等亮的竖条作为遮罩，让光带呈现垂落的丝缕感。 */}
      <pattern height="96" id={`${ids.rays}-pattern`} patternUnits="userSpaceOnUse" width="124">
        <rect fill="#fff" fillOpacity="0.35" height="96" width="124" />
        <rect fill="#fff" fillOpacity="1" height="96" width="5" x="0" />
        <rect fill="#fff" fillOpacity="0.6" height="96" width="2" x="7" />
        <rect fill="#fff" fillOpacity="0.9" height="96" width="7" x="11" />
        <rect fill="#fff" fillOpacity="0.5" height="96" width="3" x="21" />
        <rect fill="#fff" fillOpacity="1" height="96" width="4" x="27" />
        <rect fill="#fff" fillOpacity="0.75" height="96" width="9" x="35" />
        <rect fill="#fff" fillOpacity="0.55" height="96" width="2" x="47" />
        <rect fill="#fff" fillOpacity="1" height="96" width="5" x="52" />
        <rect fill="#fff" fillOpacity="0.65" height="96" width="3" x="60" />
        <rect fill="#fff" fillOpacity="0.85" height="96" width="8" x="66" />
        <rect fill="#fff" fillOpacity="0.5" height="96" width="2" x="78" />
        <rect fill="#fff" fillOpacity="1" height="96" width="6" x="83" />
        <rect fill="#fff" fillOpacity="0.6" height="96" width="3" x="92" />
        <rect fill="#fff" fillOpacity="0.8" height="96" width="10" x="98" />
        <rect fill="#fff" fillOpacity="1" height="96" width="4" x="111" />
        <rect fill="#fff" fillOpacity="0.5" height="96" width="2" x="118" />
      </pattern>
      <mask height="96" id={ids.rays} maskUnits="userSpaceOnUse" width="480" x="0" y="0">
        <rect fill={`url(#${ids.rays}-pattern)`} height="96" width="480" />
      </mask>
      <filter id={ids.blur} filterUnits="userSpaceOnUse" height="140" width="540" x="-30" y="-20">
        <feGaussianBlur stdDeviation="4" />
      </filter>
      <filter id={ids.softBlur} filterUnits="userSpaceOnUse" height="160" width="560" x="-40" y="-30">
        <feGaussianBlur stdDeviation="3.2" />
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

function AuroraLayer({ ids }: { ids: SkyIds }) {
  return (
    // 先用光帘遮罩切出竖向丝缕，再整体模糊，丝缕边缘才是柔的。
    <g filter={`url(#${ids.softBlur})`}>
      <g mask={`url(#${ids.rays})`}>
        {AURORA_BANDS.map((band) => (
          <path className={band.className} d={band.d} fill={`url(#${ids.aurora})`} key={band.className} />
        ))}
      </g>
    </g>
  );
}

function ConstellationLayer() {
  const point = (index: number) => CONSTELLATION_NODES[index] ?? CONSTELLATION_NODES[0];
  return (
    <g>
      <g strokeLinecap="round" strokeOpacity="0.32" strokeWidth="0.5" style={{ stroke: "var(--sidebar-sky-star)" }}>
        {CONSTELLATION_EDGES.map(([from, to]) => {
          const [x1, y1] = point(from);
          const [x2, y2] = point(to);
          return <line key={`${from}-${to}`} x1={x1} x2={x2} y1={y1} y2={y2} />;
        })}
      </g>
      <g fill="none" strokeLinecap="round" strokeWidth="0.9" style={{ stroke: "var(--sidebar-sky-signal)" }}>
        {CONSTELLATION_SIGNALS.map((path, index) => (
          <polyline
            className="sidebar-sky-signal"
            key={path.join("-")}
            pathLength={100}
            points={path.map((node) => point(node).slice(0, 2).join(",")).join(" ")}
            style={{ animationDelay: `${index * -2.6}s` }}
          />
        ))}
      </g>
      <g style={{ fill: "var(--sidebar-sky-star)" }}>
        {CONSTELLATION_NODES.map(([x, y, r], index) => (
          <circle
            className={PULSING_NODES.has(index) ? "sidebar-sky-node-pulse" : undefined}
            cx={x}
            cy={y}
            key={`${x}-${y}`}
            r={r}
          />
        ))}
      </g>
    </g>
  );
}
