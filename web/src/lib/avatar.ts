const AVATAR_PASSTHROUGH_PREFIXES = [
  "http://",
  "https://",
  "data:",
  "blob:",
  "/",
];

export type AvatarIconFamily = "agent" | "room";

export const AGENT_ICON_ID_START = 1;
export const AGENT_ICON_ID_END = 53;
export const ROOM_ICON_ID_START = 1;
export const ROOM_ICON_ID_END = 36;

export function getInitials(
  name: string | null | undefined,
  fallback = "AG",
  maxLength = 2,
): string {
  if (!name) {
    return fallback;
  }

  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return fallback;
  }
  if (parts.length === 1) {
    return parts[0].slice(0, maxLength).toUpperCase();
  }
  return parts
    .slice(0, maxLength)
    .map((part) => part[0] ?? "")
    .join("")
    .toUpperCase();
}

/** 将头像标识解析为可直接使用的图片地址。 */
export function getIconAvatarSrc(
  avatar: string | null | undefined,
  iconFamily: AvatarIconFamily = "agent",
): string | null {
  const normalizedAvatar = avatar?.trim();
  if (!normalizedAvatar) {
    return null;
  }
  if (
    AVATAR_PASSTHROUGH_PREFIXES.some(
      (prefix) => normalizedAvatar.startsWith(prefix),
    )
  ) {
    return normalizedAvatar;
  }
  return `/icon/${iconFamily}/${normalizedAvatar}.png`;
}

function getStableIconId(
  seed: string | null | undefined,
  startInclusive: number,
  endInclusive: number,
): string {
  const normalizedSeed = seed?.trim() || "nexus";
  const range = endInclusive - startInclusive + 1;
  let hash = 0;

  for (let index = 0; index < normalizedSeed.length; index += 1) {
    hash = (hash * 31 + normalizedSeed.charCodeAt(index)) >>> 0;
  }
  return String(startInclusive + (hash % range));
}

/** 未配置头像的 Room 使用稳定编号，避免重渲染时改变视觉身份。 */
export function getRoomAvatarIconId(
  roomId: string | null | undefined,
  roomName: string | null | undefined,
  explicitAvatar?: string | null,
): string {
  return explicitAvatar?.trim() || getStableIconId(
    roomId || roomName,
    ROOM_ICON_ID_START,
    ROOM_ICON_ID_END,
  );
}
