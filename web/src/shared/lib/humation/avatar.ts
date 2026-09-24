// INPUT: 版本化头像短标识或显式部件和颜色。
// OUTPUT: 经白名单校验的头像配置和本地 SVG 图片地址。
// POS: 内置 Humation 的唯一持久化与渲染适配，不接受外部 SVG。
import { manifest } from "./assets";
import { createAvatar, fnv1a } from "./create-avatar";
import { nexusParts } from "./nexus-parts";
import type { AvatarJson } from "./types";

export const avatarAssets = { ...manifest, parts: [...manifest.parts, ...nexusParts] };
// 字段顺序属于 h1 持久协议，不能随素材目录排序改变。
export const avatarSlots = ["head", "body", "bottom", "item", "glasses"] as const;
export const avatarColors = ["hair", "skin", "clothes", "bottom", "stroke"] as const;

// 背景属于头像素材配色，按独立种子选色并随部件一起持久化。
const BACKGROUND_COLORS = [
  "8B3DFF", "B5A1F7", "96C9F4", "8ED9C4",
  "F3CE75", "F6A6B2", "F5B58C", "C2D98B",
] as const;

export function createAvatarState(seed: string): AvatarJson {
  const background = BACKGROUND_COLORS[fnv1a(`${seed}:background`) % BACKGROUND_COLORS.length];
  return createAvatar(avatarAssets, { seed, background }).toJSON();
}

export function encodeAvatar(state: AvatarJson): string {
  return `h1:${[
    ...avatarSlots.map((slot) => state.selections[slot]),
    ...avatarColors.map((color) => state.colors[color].replace(/^#/, "").toUpperCase()),
    state.background.replace(/^#/, "").toUpperCase(),
  ].join(".")}`;
}

export function decodeAvatar(value: string): AvatarJson | null {
  if (!value.startsWith("h1:") || value.length > 255) return null;
  const fields = value.slice(3).split(".");
  if (fields.length !== 11) return null;
  const selections: Record<string, string> = {};
  for (const [index, slot] of avatarSlots.entries()) {
    const id = fields[index];
    if (!avatarAssets.parts.some((part) => part.id === id && part.selectionSlot === slot)) return null;
    selections[slot] = id;
  }
  if (!fields.slice(5).every((color) => /^[0-9a-f]{6}$/i.test(color))) return null;
  return createAvatar(avatarAssets, {
    selections,
    colors: Object.fromEntries(avatarColors.map((color, index) => [color, fields[index + 5]])),
    background: fields[10],
  }).toJSON();
}

const imageCache = new Map<string, string>();

export function getHumationAvatarSrc(value: string): string | null {
  const cached = imageCache.get(value);
  if (cached) return cached;
  const state = decodeAvatar(value);
  if (!state) return null;
  const src = createAvatar(avatarAssets, state).toDataUri();
  if (imageCache.size >= 128) imageCache.delete(imageCache.keys().next().value!);
  imageCache.set(value, src);
  return src;
}

/** 随机组合只在用户操作或创建草稿时生成，保存后保持身份稳定。 */
export function getRandomHumationAvatar(): string {
  return encodeAvatar(createAvatarState(crypto.randomUUID()));
}

/** 子智能体按精确工具/任务身份生成，不随名称和执行状态变化。 */
export function getSeededHumationAvatarSrc(seed: string): string {
  return getHumationAvatarSrc(encodeAvatar(createAvatarState(seed.trim() || "nexus")))!;
}
