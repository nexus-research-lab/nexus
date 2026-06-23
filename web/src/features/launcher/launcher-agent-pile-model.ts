import { SpotlightToken } from "@/types/app/launcher";

export type TokenPhysicsConfig = {
  key: string;
  size: number;
  radius: number;
  spawn_x: number;
  spawn_y: number;
  angle: number;
  delay: number;
};

export type TokenBrandStyle = {
  label_class_name: string;
  label_transform: string;
  tag: string;
  tag_class_name: string;
  tag_opacity: number;
  rotation_class_name: string;
  inner_inset: number;
  inner_radius: string;
  accent_opacity: number;
  gloss_opacity: number;
  fold: boolean;
  stacked: boolean;
  ring: boolean;
};

export function create_token_config(tokens: SpotlightToken[], width: number): TokenPhysicsConfig[] {
  const horizontalPadding = 108;
  return tokens.map((token, index) => {
    const seed = hash_string(token.key);
    const baseSize = token.kind === "agent" ? 40 : 44;
    const size = baseSize + Math.round(seeded_unit(seed, 1) * 12);
    return {
      key: token.key,
      size,
      radius: token.kind === "agent" ? size / 2 : Math.max(12, Math.round(size * 0.28)),
      spawn_x:
        horizontalPadding + seeded_unit(seed, 2) * Math.max(width - horizontalPadding * 2, 72),
      spawn_y: -180 - seeded_unit(seed, 3) * 240 - index * 14,
      angle: ((seeded_unit(seed, 4) * 36 - 18) * Math.PI) / 180,
      delay: 40 + index * 55,
    };
  });
}

export function hex_to_rgba(hex: string, alpha: number) {
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

export function get_token_brand_style(token: SpotlightToken): TokenBrandStyle {
  const hash = hash_string(token.key);
  const variant = hash % 5;

  if (variant === 0) {
    return {
      label_class_name: token.label.length >= 3 ? "text-2xs tracking-[-0.03em]" : "text-sm tracking-[-0.08em]",
      label_transform: "none",
      tag: token.kind === "agent" ? "core" : "room",
      tag_class_name: "text-[6px] tracking-[0.2em]",
      tag_opacity: 0.62,
      rotation_class_name: "",
      inner_inset: 2,
      inner_radius: token.kind === "agent" ? "9999px" : "12px",
      accent_opacity: 0.2,
      gloss_opacity: 0.38,
      fold: false,
      stacked: false,
      ring: true,
    };
  }

  if (variant === 1) {
    return {
      label_class_name: token.label.length >= 3 ? "text-[8px] tracking-[0.04em]" : "text-sm tracking-[0.08em]",
      label_transform: "uppercase",
      tag: token.kind === "agent" ? "lab" : "sync",
      tag_class_name: "text-[6px] tracking-[0.24em]",
      tag_opacity: 0.54,
      rotation_class_name: "rotate-[-4deg]",
      inner_inset: 2,
      inner_radius: token.kind === "agent" ? "9999px" : "11px",
      accent_opacity: 0.26,
      gloss_opacity: 0.32,
      fold: token.kind === "room",
      stacked: false,
      ring: false,
    };
  }

  if (variant === 2) {
    return {
      label_class_name: token.label.length >= 3 ? "text-2xs tracking-[-0.08em]" : "text-base tracking-[-0.1em]",
      label_transform: "none",
      tag: token.kind === "agent" ? "net" : "grid",
      tag_class_name: "text-[6px] tracking-[0.16em]",
      tag_opacity: 0.58,
      rotation_class_name: token.kind === "room" ? "rotate-[-8deg]" : "",
      inner_inset: 2,
      inner_radius: token.kind === "agent" ? "9999px" : "12px",
      accent_opacity: 0.18,
      gloss_opacity: 0.34,
      fold: false,
      stacked: token.kind === "room",
      ring: false,
    };
  }

  if (variant === 3) {
    return {
      label_class_name: get_label_size(token.label),
      label_transform: "capitalize",
      tag: token.kind === "agent" ? "ai" : "hub",
      tag_class_name: "text-[6px] tracking-[0.28em]",
      tag_opacity: 0.48,
      rotation_class_name: "rotate-[3deg]",
      inner_inset: 1.5,
      inner_radius: token.kind === "agent" ? "9999px" : "13px",
      accent_opacity: 0.24,
      gloss_opacity: 0.3,
      fold: hash % 2 === 0,
      stacked: false,
      ring: false,
    };
  }

  return {
    label_class_name: token.label.length >= 3 ? "text-[8px] tracking-[0.12em]" : "text-xs tracking-[0.16em]",
    label_transform: "uppercase",
    tag: token.kind === "agent" ? "os" : "flow",
    tag_class_name: "text-[5px] tracking-[0.3em]",
    tag_opacity: 0.42,
    rotation_class_name: token.kind === "room" ? "rotate-[6deg]" : "rotate-[-2deg]",
    inner_inset: 2.5,
    inner_radius: token.kind === "agent" ? "9999px" : "10px",
    accent_opacity: 0.22,
    gloss_opacity: 0.26,
    fold: false,
    stacked: true,
    ring: hash % 2 === 1,
  };
}

function seeded_unit(seed: number, salt: number) {
  const value = Math.sin(seed * 12.9898 + salt * 78.233) * 43758.5453;
  return value - Math.floor(value);
}

function get_label_size(label: string) {
  if (label.length >= 3) {
    return "text-2xs";
  }
  return "text-sm";
}

function hash_string(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
