import type { TourPlacement } from "./tour-overlay-geometry";

type TourStickerPlacement = "hang" | "perch" | "peek" | "point" | "hold";

export interface TourStickerAsset {
  placement: TourStickerPlacement;
  src: string;
}

const TOUR_STICKERS: readonly TourStickerAsset[] = [
  { src: "/nexus/stickers/card-top.png", placement: "perch" },
  { src: "/nexus/stickers/hanging.png", placement: "hang" },
  { src: "/nexus/stickers/peek-right.png", placement: "peek" },
  { src: "/nexus/stickers/pointing.png", placement: "point" },
  { src: "/nexus/stickers/holding-card.png", placement: "hold" },
];

const FIXED_STICKER_INDEX: Partial<Record<TourPlacement, number>> = {
  center: 0,
  left: 2,
};

const STICKER_CLASS_NAMES: Record<TourStickerPlacement, string> = {
  hang: "-top-12 right-7 h-[72px] w-auto",
  hold: "top-1/2 -right-10 h-[82px] w-auto -translate-y-1/2",
  peek: "top-16 -left-10 h-[82px] w-auto",
  perch: "-top-10 left-14 h-[74px] w-auto -translate-x-1/2",
  point: "-top-[52px] right-4 h-[72px] w-auto",
};

const TOP_CLEARANCE_STICKERS = new Set<TourStickerPlacement>([
  "hang",
  "perch",
  "point",
]);

export function resolveTourSticker(
  stepIndex: number,
  placement: TourPlacement,
): TourStickerAsset {
  const index = FIXED_STICKER_INDEX[placement]
    ?? ((stepIndex + 1) % TOUR_STICKERS.length);
  return TOUR_STICKERS[index];
}

export function getStickerTopClearance(sticker: TourStickerAsset): number {
  return TOP_CLEARANCE_STICKERS.has(sticker.placement) ? 72 : 16;
}

export function getStickerClassName(sticker: TourStickerAsset): string {
  return STICKER_CLASS_NAMES[sticker.placement];
}
