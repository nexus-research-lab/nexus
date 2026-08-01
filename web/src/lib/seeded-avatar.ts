export type SeededAvatarIcon =
  | "asterisk"
  | "box"
  | "braces"
  | "circle-dot"
  | "diamond"
  | "orbit"
  | "sparkles"
  | "zap";

export interface SeededAvatarAppearance {
  backgroundColor: string;
  foregroundColor: string;
  icon: SeededAvatarIcon;
}

const SEEDED_AVATAR_PALETTES = [
  { backgroundColor: "#E8F39A", foregroundColor: "#182109" },
  { backgroundColor: "#DCEAFF", foregroundColor: "#17365F" },
  { backgroundColor: "#FFE1C4", foregroundColor: "#542B0F" },
  { backgroundColor: "#E8DFFF", foregroundColor: "#382560" },
  { backgroundColor: "#D8F0E5", foregroundColor: "#173E2D" },
  { backgroundColor: "#FFDDE3", foregroundColor: "#57212D" },
  { backgroundColor: "#F8E6A8", foregroundColor: "#49370E" },
  { backgroundColor: "#D7EFF2", foregroundColor: "#173B41" },
] as const;

const SEEDED_AVATAR_ICONS: readonly SeededAvatarIcon[] = [
  "zap",
  "sparkles",
  "braces",
  "circle-dot",
  "diamond",
  "orbit",
  "asterisk",
  "box",
];

/** 中文注释：使用稳定散列代替随机数，确保同一标识在所有页面保持同一头像。 */
function hashSeed(value: string): number {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

export function getSeededAvatarAppearance(
  seed: string,
): SeededAvatarAppearance {
  const normalizedSeed = seed.trim().toLowerCase() || "nexus";
  const palette = SEEDED_AVATAR_PALETTES[
    hashSeed(`palette:${normalizedSeed}`) % SEEDED_AVATAR_PALETTES.length
  ];
  const icon = SEEDED_AVATAR_ICONS[
    hashSeed(`icon:${normalizedSeed}`) % SEEDED_AVATAR_ICONS.length
  ];
  return {
    backgroundColor: palette.backgroundColor,
    foregroundColor: palette.foregroundColor,
    icon,
  };
}
