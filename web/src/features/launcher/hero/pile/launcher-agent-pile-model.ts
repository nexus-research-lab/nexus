import type { SpotlightToken } from "@/types/app/launcher";

type TokenKind = SpotlightToken["kind"];

export interface TokenPhysicsConfig {
  angle: number;
  delay: number;
  key: string;
  radius: number;
  size: number;
  spawnX: number;
  spawnY: number;
}

export interface TokenBrandStyle {
  glossOpacity: number;
  innerInset: number;
  innerRadius: string;
  labelClassName: string;
  labelTransform: "none" | "uppercase" | "capitalize";
  rotationClassName: string;
  tag: string;
  tagClassName: string;
  tagOpacity: number;
}

interface TokenBrandVariant {
  glossOpacity: number;
  innerInset: number;
  innerRadius: Readonly<Record<TokenKind, string>>;
  labelClassName: {
    long: string;
    short: string;
  };
  labelTransform: TokenBrandStyle["labelTransform"];
  rotationClassName: Readonly<Record<TokenKind, string>>;
  tag: Readonly<Record<TokenKind, string>>;
  tagClassName: string;
  tagOpacity: number;
}

const TOKEN_BASE_SIZE: Readonly<Record<TokenKind, number>> = {
  agent: 40,
  room: 44,
};

const BRAND_VARIANTS: readonly TokenBrandVariant[] = [
  {
    glossOpacity: 0.38,
    innerInset: 2,
    innerRadius: { agent: "9999px", room: "12px" },
    labelClassName: {
      long: "text-2xs tracking-[-0.03em]",
      short: "text-sm tracking-[-0.08em]",
    },
    labelTransform: "none",
    rotationClassName: { agent: "", room: "" },
    tag: { agent: "core", room: "room" },
    tagClassName: "text-[6px] tracking-[0.2em]",
    tagOpacity: 0.62,
  },
  {
    glossOpacity: 0.32,
    innerInset: 2,
    innerRadius: { agent: "9999px", room: "11px" },
    labelClassName: {
      long: "text-[8px] tracking-[0.04em]",
      short: "text-sm tracking-[0.08em]",
    },
    labelTransform: "uppercase",
    rotationClassName: { agent: "rotate-[-4deg]", room: "rotate-[-4deg]" },
    tag: { agent: "lab", room: "sync" },
    tagClassName: "text-[6px] tracking-[0.24em]",
    tagOpacity: 0.54,
  },
  {
    glossOpacity: 0.34,
    innerInset: 2,
    innerRadius: { agent: "9999px", room: "12px" },
    labelClassName: {
      long: "text-2xs tracking-[-0.08em]",
      short: "text-base tracking-[-0.1em]",
    },
    labelTransform: "none",
    rotationClassName: { agent: "", room: "rotate-[-8deg]" },
    tag: { agent: "net", room: "grid" },
    tagClassName: "text-[6px] tracking-[0.16em]",
    tagOpacity: 0.58,
  },
  {
    glossOpacity: 0.3,
    innerInset: 1.5,
    innerRadius: { agent: "9999px", room: "13px" },
    labelClassName: { long: "text-2xs", short: "text-sm" },
    labelTransform: "capitalize",
    rotationClassName: { agent: "rotate-[3deg]", room: "rotate-[3deg]" },
    tag: { agent: "ai", room: "hub" },
    tagClassName: "text-[6px] tracking-[0.28em]",
    tagOpacity: 0.48,
  },
  {
    glossOpacity: 0.26,
    innerInset: 2.5,
    innerRadius: { agent: "9999px", room: "10px" },
    labelClassName: {
      long: "text-[8px] tracking-[0.12em]",
      short: "text-xs tracking-[0.16em]",
    },
    labelTransform: "uppercase",
    rotationClassName: { agent: "rotate-[-2deg]", room: "rotate-[6deg]" },
    tag: { agent: "os", room: "flow" },
    tagClassName: "text-[5px] tracking-[0.3em]",
    tagOpacity: 0.42,
  },
];

export function createTokenConfig(
  tokens: SpotlightToken[],
  width: number,
): TokenPhysicsConfig[] {
  const horizontalPadding = 108;
  return tokens.map((token, index) => {
    const seed = hashString(token.key);
    const size = TOKEN_BASE_SIZE[token.kind] + Math.round(seededUnit(seed, 1) * 12);
    const radiusByKind: Readonly<Record<TokenKind, number>> = {
      agent: size / 2,
      room: Math.max(12, Math.round(size * 0.28)),
    };
    return {
      angle: ((seededUnit(seed, 4) * 36 - 18) * Math.PI) / 180,
      delay: 40 + index * 55,
      key: token.key,
      radius: radiusByKind[token.kind],
      size,
      spawnX: horizontalPadding
        + seededUnit(seed, 2) * Math.max(width - horizontalPadding * 2, 72),
      spawnY: -180 - seededUnit(seed, 3) * 240 - index * 14,
    };
  });
}

export function getTokenBrandStyle(token: SpotlightToken): TokenBrandStyle {
  const variant = BRAND_VARIANTS[hashString(token.key) % BRAND_VARIANTS.length];
  return {
    glossOpacity: variant.glossOpacity,
    innerInset: variant.innerInset,
    innerRadius: variant.innerRadius[token.kind],
    labelClassName: token.label.length >= 3
      ? variant.labelClassName.long
      : variant.labelClassName.short,
    labelTransform: variant.labelTransform,
    rotationClassName: variant.rotationClassName[token.kind],
    tag: variant.tag[token.kind],
    tagClassName: variant.tagClassName,
    tagOpacity: variant.tagOpacity,
  };
}

export function hexToRgba(hex: string, alpha: number): string {
  const normalized = hex.replace("#", "");
  const value = normalized.length === 3
    ? normalized
      .split("")
      .map((item) => `${item}${item}`)
      .join("")
    : normalized;
  const red = Number.parseInt(value.slice(0, 2), 16);
  const green = Number.parseInt(value.slice(2, 4), 16);
  const blue = Number.parseInt(value.slice(4, 6), 16);
  return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

function seededUnit(seed: number, salt: number): number {
  const value = Math.sin(seed * 12.9898 + salt * 78.233) * 43758.5453;
  return value - Math.floor(value);
}

function hashString(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
