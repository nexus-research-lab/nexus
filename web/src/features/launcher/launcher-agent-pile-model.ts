import { SpotlightToken } from "@/types/app/launcher";

export type TokenPhysicsConfig = {
  key: string;
  size: number;
  radius: number;
  spawnX: number;
  spawnY: number;
  angle: number;
  delay: number;
};

export type TokenBrandStyle = {
  labelClassName: string;
  labelTransform: string;
  tag: string;
  tagClassName: string;
  tagOpacity: number;
  rotationClassName: string;
  innerInset: number;
  innerRadius: string;
  accentOpacity: number;
  glossOpacity: number;
  fold: boolean;
  stacked: boolean;
  ring: boolean;
};

export function createTokenConfig(tokens: SpotlightToken[], width: number): TokenPhysicsConfig[] {
  const horizontalPadding = 108;
  return tokens.map((token, index) => {
    const seed = hashString(token.key);
    const baseSize = token.kind === "agent" ? 40 : 44;
    const size = baseSize + Math.round(seededUnit(seed, 1) * 12);
    return {
      key: token.key,
      size,
      radius: token.kind === "agent" ? size / 2 : Math.max(12, Math.round(size * 0.28)),
      spawnX:
        horizontalPadding + seededUnit(seed, 2) * Math.max(width - horizontalPadding * 2, 72),
      spawnY: -180 - seededUnit(seed, 3) * 240 - index * 14,
      angle: ((seededUnit(seed, 4) * 36 - 18) * Math.PI) / 180,
      delay: 40 + index * 55,
    };
  });
}

export function hexToRgba(hex: string, alpha: number) {
  const normalized = hex.replace("#", "");
  const value =
    normalized.length === 3
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

export function getTokenBrandStyle(token: SpotlightToken): TokenBrandStyle {
  const hash = hashString(token.key);
  const variant = hash % 5;

  if (variant === 0) {
    return {
      labelClassName: token.label.length >= 3 ? "text-2xs tracking-[-0.03em]" : "text-sm tracking-[-0.08em]",
      labelTransform: "none",
      tag: token.kind === "agent" ? "core" : "room",
      tagClassName: "text-[6px] tracking-[0.2em]",
      tagOpacity: 0.62,
      rotationClassName: "",
      innerInset: 2,
      innerRadius: token.kind === "agent" ? "9999px" : "12px",
      accentOpacity: 0.2,
      glossOpacity: 0.38,
      fold: false,
      stacked: false,
      ring: true,
    };
  }

  if (variant === 1) {
    return {
      labelClassName: token.label.length >= 3 ? "text-[8px] tracking-[0.04em]" : "text-sm tracking-[0.08em]",
      labelTransform: "uppercase",
      tag: token.kind === "agent" ? "lab" : "sync",
      tagClassName: "text-[6px] tracking-[0.24em]",
      tagOpacity: 0.54,
      rotationClassName: "rotate-[-4deg]",
      innerInset: 2,
      innerRadius: token.kind === "agent" ? "9999px" : "11px",
      accentOpacity: 0.26,
      glossOpacity: 0.32,
      fold: token.kind === "room",
      stacked: false,
      ring: false,
    };
  }

  if (variant === 2) {
    return {
      labelClassName: token.label.length >= 3 ? "text-2xs tracking-[-0.08em]" : "text-base tracking-[-0.1em]",
      labelTransform: "none",
      tag: token.kind === "agent" ? "net" : "grid",
      tagClassName: "text-[6px] tracking-[0.16em]",
      tagOpacity: 0.58,
      rotationClassName: token.kind === "room" ? "rotate-[-8deg]" : "",
      innerInset: 2,
      innerRadius: token.kind === "agent" ? "9999px" : "12px",
      accentOpacity: 0.18,
      glossOpacity: 0.34,
      fold: false,
      stacked: token.kind === "room",
      ring: false,
    };
  }

  if (variant === 3) {
    return {
      labelClassName: getLabelSize(token.label),
      labelTransform: "capitalize",
      tag: token.kind === "agent" ? "ai" : "hub",
      tagClassName: "text-[6px] tracking-[0.28em]",
      tagOpacity: 0.48,
      rotationClassName: "rotate-[3deg]",
      innerInset: 1.5,
      innerRadius: token.kind === "agent" ? "9999px" : "13px",
      accentOpacity: 0.24,
      glossOpacity: 0.3,
      fold: hash % 2 === 0,
      stacked: false,
      ring: false,
    };
  }

  return {
    labelClassName: token.label.length >= 3 ? "text-[8px] tracking-[0.12em]" : "text-xs tracking-[0.16em]",
    labelTransform: "uppercase",
    tag: token.kind === "agent" ? "os" : "flow",
    tagClassName: "text-[5px] tracking-[0.3em]",
    tagOpacity: 0.42,
    rotationClassName: token.kind === "room" ? "rotate-[6deg]" : "rotate-[-2deg]",
    innerInset: 2.5,
    innerRadius: token.kind === "agent" ? "9999px" : "10px",
    accentOpacity: 0.22,
    glossOpacity: 0.26,
    fold: false,
    stacked: true,
    ring: hash % 2 === 1,
  };
}

function seededUnit(seed: number, salt: number) {
  const value = Math.sin(seed * 12.9898 + salt * 78.233) * 43758.5453;
  return value - Math.floor(value);
}

function getLabelSize(label: string) {
  if (label.length >= 3) {
    return "text-2xs";
  }
  return "text-sm";
}

function hashString(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
