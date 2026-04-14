/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：glass-slider.tsx
# @Date   ：2026-04-12 21:27
# @Author ：leemysw
# 2026-04-12 21:27   Create
# =====================================================
*/

"use client";

import { cn } from "@/lib/utils";

interface GlassSliderProps {
  value: number;
  min?: number;
  max?: number;
  step?: number;
  disabled?: boolean;
  class_name?: string;
  on_change: (value: number) => void;
}

/**
 * 中文注释：共享 glass 滑杆，采用亮绿色轨道和轻凹槽表现，
 * 作为后续高关注滑杆控件的统一原语。
 */
export function GlassSlider({
  value,
  min = 0,
  max = 100,
  step = 1,
  disabled = false,
  class_name,
  on_change,
}: GlassSliderProps) {
  const percentage = ((value - min) / Math.max(max - min, 1)) * 100;

  return (
    <div className={cn("relative flex h-8 items-center", class_name)}>
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-1/2 h-4 -translate-y-1/2 rounded-full border border-[color:color-mix(in_srgb,rgba(172,255,190,0.56)_78%,rgba(255,255,255,0.3))]"
        style={{
          background:
            "linear-gradient(180deg, rgba(255,255,255,0.34), rgba(255,255,255,0.1)), linear-gradient(90deg, rgba(76,207,122,0.30), rgba(76,207,122,0.12))",
          boxShadow:
            "inset 0 2px 6px rgba(255,255,255,0.18), inset 0 -7px 14px rgba(11,90,35,0.18), 0 10px 22px rgba(40,156,71,0.10)",
        }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute left-0 top-1/2 h-4 -translate-y-1/2 rounded-full"
        style={{
          width: `${percentage}%`,
          background:
            "linear-gradient(90deg, rgba(76,207,122,0.78), rgba(110,228,140,0.62))",
          boxShadow: "inset 0 1px 0 rgba(255,255,255,0.28)",
        }}
      />
      <input
        className="relative z-[1] h-8 w-full appearance-none bg-transparent outline-none disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) [&::-webkit-slider-runnable-track]:h-4 [&::-webkit-slider-runnable-track]:appearance-none [&::-webkit-slider-runnable-track]:bg-transparent [&::-webkit-slider-thumb]:mt-[-2px] [&::-webkit-slider-thumb]:h-5 [&::-webkit-slider-thumb]:w-5 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:border [&::-webkit-slider-thumb]:border-[rgba(255,255,255,0.68)] [&::-webkit-slider-thumb]:bg-[linear-gradient(180deg,rgba(240,255,242,0.98),rgba(199,255,214,0.94))] [&::-webkit-slider-thumb]:shadow-[0_8px_18px_rgba(44,153,68,0.22)]"
        disabled={disabled}
        max={max}
        min={min}
        onChange={(event) => on_change(Number(event.target.value))}
        step={step}
        type="range"
        value={value}
      />
    </div>
  );
}
