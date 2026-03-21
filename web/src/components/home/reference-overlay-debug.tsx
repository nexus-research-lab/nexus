"use client";

import { type PointerEvent as ReactPointerEvent, useCallback, useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

import { useBlobDebugEnabled } from "@/components/home/hero-blob-debug-hooks";

const DEBUG_REFERENCE_IMAGE = "/debug/nexus-collaboration-hub-home.png";
const DEBUG_REFERENCE_STORAGE_KEY = "nexus-home-reference-overlay";

interface ReferenceOverlayState {
  offsetX: number;
  offsetY: number;
  opacity: number;
  scale: number;
  layerMode: "front" | "back";
}

const DEFAULT_REFERENCE_OVERLAY: ReferenceOverlayState = {
  offsetX: 0,
  offsetY: -40,
  opacity: 0.7,
  scale: 0.82,
  layerMode: "front",
};

function parseReferenceOverlay(raw: string | null): ReferenceOverlayState | null {
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<ReferenceOverlayState>;
    if (
      typeof parsed.offsetX !== "number" ||
      typeof parsed.offsetY !== "number" ||
      typeof parsed.opacity !== "number" ||
      typeof parsed.scale !== "number" ||
      (parsed.layerMode !== "front" && parsed.layerMode !== "back")
    ) {
      return null;
    }

    return {
      offsetX: parsed.offsetX,
      offsetY: parsed.offsetY,
      opacity: Math.min(1, Math.max(0.1, parsed.opacity)),
      scale: Math.min(1.6, Math.max(0.3, parsed.scale)),
      layerMode: parsed.layerMode,
    };
  } catch {
    return null;
  }
}

export function DebugReferenceOverlay() {
  const debugEnabled = useBlobDebugEnabled();
  const dragPointerIdRef = useRef<number | null>(null);
  const dragStartRef = useRef<{ pointerX: number; pointerY: number; startX: number; startY: number } | null>(null);
  const [overlay, setOverlay] = useState<ReferenceOverlayState>(DEFAULT_REFERENCE_OVERLAY);

  useEffect(() => {
    if (!debugEnabled) {
      return;
    }

    const persisted = parseReferenceOverlay(window.localStorage.getItem(DEBUG_REFERENCE_STORAGE_KEY));
    if (persisted) {
      setOverlay(persisted);
    }
  }, [debugEnabled]);

  useEffect(() => {
    if (!debugEnabled) {
      return;
    }

    window.localStorage.setItem(DEBUG_REFERENCE_STORAGE_KEY, JSON.stringify(overlay));
  }, [debugEnabled, overlay]);

  useEffect(() => {
    if (!debugEnabled) {
      return;
    }

    const handlePointerMove = (event: PointerEvent) => {
      if (dragPointerIdRef.current === null || dragPointerIdRef.current !== event.pointerId || !dragStartRef.current) {
        return;
      }

      const deltaX = event.clientX - dragStartRef.current.pointerX;
      const deltaY = event.clientY - dragStartRef.current.pointerY;
      setOverlay((current) => ({
        ...current,
        offsetX: dragStartRef.current!.startX + deltaX,
        offsetY: dragStartRef.current!.startY + deltaY,
      }));
    };

    const handlePointerUp = (event: PointerEvent) => {
      if (dragPointerIdRef.current !== event.pointerId) {
        return;
      }

      dragPointerIdRef.current = null;
      dragStartRef.current = null;
    };

    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    window.addEventListener("pointercancel", handlePointerUp);

    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      window.removeEventListener("pointercancel", handlePointerUp);
    };
  }, [debugEnabled]);

  const handleDragStart = useCallback((event: ReactPointerEvent<HTMLButtonElement>) => {
    dragPointerIdRef.current = event.pointerId;
    dragStartRef.current = {
      pointerX: event.clientX,
      pointerY: event.clientY,
      startX: overlay.offsetX,
      startY: overlay.offsetY,
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  }, [overlay.offsetX, overlay.offsetY]);

  const nudgeOverlay = useCallback((deltaX: number, deltaY: number) => {
    setOverlay((current) => ({
      ...current,
      offsetX: current.offsetX + deltaX,
      offsetY: current.offsetY + deltaY,
    }));
  }, []);

  if (!debugEnabled) {
    return null;
  }

  return (
    <>
      <div
        className={cn(
          "pointer-events-none absolute inset-0 overflow-visible",
          overlay.layerMode === "front" ? "z-[11]" : "z-[2]",
        )}
      >
        <div className="absolute inset-0 flex justify-center overflow-visible">
          <div
            className="relative origin-top"
            style={{
              transform: `translate3d(${overlay.offsetX}px, ${overlay.offsetY}px, 0) scale(${overlay.scale})`,
              width: "1366px",
            }}
          >
            <img
              alt="Reference overlay"
              className="h-auto w-full select-none rounded-[28px] object-contain shadow-[0_24px_80px_rgba(0,0,0,0.35)]"
              draggable={false}
              src={DEBUG_REFERENCE_IMAGE}
              style={{ opacity: overlay.opacity }}
            />
          </div>
        </div>
      </div>

      <div className="absolute inset-0 z-[14] overflow-visible">
        <button
          className="absolute left-1/2 top-0 flex h-7 w-7 items-center justify-center rounded-full border border-cyan-200/70 bg-cyan-400/45 text-[10px] font-semibold text-white shadow-[0_8px_18px_rgba(34,211,238,0.26)] backdrop-blur-sm"
          onPointerDown={handleDragStart}
          style={{
            transform: `translate3d(calc(-50% + ${overlay.offsetX}px), ${overlay.offsetY}px, 0)`,
            touchAction: "none",
          }}
          type="button"
        >
          拖
        </button>

        <div className="absolute -right-20 top-1 w-[300px] rounded-2xl border border-white/18 bg-black/72 p-4 text-white shadow-[0_18px_50px_rgba(0,0,0,0.35)] backdrop-blur-xl">
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-white/74">Reference</p>
          <p className="mt-1 text-[11px] leading-5 text-white/46">拖动小圆点对齐参考图，再按轮廓描边。</p>

          <div className="mt-3 flex items-center gap-2">
            <button
              className={cn(
                "rounded-full border px-3 py-1.5 text-[11px] transition-colors",
                overlay.layerMode === "front"
                  ? "border-cyan-200/40 bg-cyan-300/12 text-white"
                  : "border-white/14 text-white/62 hover:text-white",
              )}
              onClick={() => setOverlay((current) => ({ ...current, layerMode: "front" }))}
              type="button"
            >
              置前参考
            </button>
            <button
              className={cn(
                "rounded-full border px-3 py-1.5 text-[11px] transition-colors",
                overlay.layerMode === "back"
                  ? "border-cyan-200/40 bg-cyan-300/12 text-white"
                  : "border-white/14 text-white/62 hover:text-white",
              )}
              onClick={() => setOverlay((current) => ({ ...current, layerMode: "back" }))}
              type="button"
            >
              置后参考
            </button>
          </div>

          <div className="mt-3 grid grid-cols-3 gap-2">
            <div />
            <button
              className="rounded-xl border border-white/14 bg-white/8 px-3 py-2 text-xs text-white/72 transition-colors hover:bg-white/14 hover:text-white"
              onClick={() => nudgeOverlay(0, -10)}
              type="button"
            >
              上
            </button>
            <div />
            <button
              className="rounded-xl border border-white/14 bg-white/8 px-3 py-2 text-xs text-white/72 transition-colors hover:bg-white/14 hover:text-white"
              onClick={() => nudgeOverlay(-10, 0)}
              type="button"
            >
              左
            </button>
            <button
              className="rounded-xl border border-white/14 bg-white/8 px-3 py-2 text-xs text-white/72 transition-colors hover:bg-white/14 hover:text-white"
              onClick={() => setOverlay(DEFAULT_REFERENCE_OVERLAY)}
              type="button"
            >
              归位
            </button>
            <button
              className="rounded-xl border border-white/14 bg-white/8 px-3 py-2 text-xs text-white/72 transition-colors hover:bg-white/14 hover:text-white"
              onClick={() => nudgeOverlay(10, 0)}
              type="button"
            >
              右
            </button>
            <div />
            <button
              className="rounded-xl border border-white/14 bg-white/8 px-3 py-2 text-xs text-white/72 transition-colors hover:bg-white/14 hover:text-white"
              onClick={() => nudgeOverlay(0, 10)}
              type="button"
            >
              下
            </button>
            <div />
          </div>

          <div className="mt-3 grid grid-cols-2 gap-2 text-[11px] text-white/54">
            <div className="rounded-xl border border-white/10 bg-white/4 px-3 py-2">
              X: {Math.round(overlay.offsetX)} px
            </div>
            <div className="rounded-xl border border-white/10 bg-white/4 px-3 py-2">
              Y: {Math.round(overlay.offsetY)} px
            </div>
          </div>

          <label className="mt-3 block text-[11px] text-white/54">
            缩放 {overlay.scale.toFixed(2)}
            <input
              className="mt-1 w-full accent-cyan-300"
              max="1.3"
              min="0.45"
              onChange={(event) => setOverlay((current) => ({ ...current, scale: Number(event.target.value) }))}
              step="0.01"
              type="range"
              value={overlay.scale}
            />
          </label>

          <label className="mt-3 block text-[11px] text-white/54">
            透明度 {overlay.opacity.toFixed(2)}
            <input
              className="mt-1 w-full accent-cyan-300"
              max="0.95"
              min="0.2"
              onChange={(event) => setOverlay((current) => ({ ...current, opacity: Number(event.target.value) }))}
              step="0.01"
              type="range"
              value={overlay.opacity}
            />
          </label>
        </div>
      </div>
    </>
  );
}
