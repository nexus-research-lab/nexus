import { cn } from "@/shared/ui/class-name";

import {
  getStickerClassName,
  type TourStickerAsset,
} from "./tour-overlay-sticker";

export function TourStepSticker({ sticker }: { sticker: TourStickerAsset }) {
  return (
    <img
      alt=""
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute z-20 select-none drop-shadow-[0_14px_20px_rgba(68,74,120,0.12)] max-[520px]:hidden",
        getStickerClassName(sticker),
      )}
      src={sticker.src}
    />
  );
}
