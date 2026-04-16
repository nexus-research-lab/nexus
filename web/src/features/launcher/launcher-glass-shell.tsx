/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：launcher-glass-shell.tsx
# @Date   ：2026-04-12 19:43
# @Author ：leemysw
# 2026-04-12 19:43   Create
# =====================================================
*/

"use client";

import { ReactNode, useId } from "react";

import {
  create_closed_spline_path,
  create_inner_points,
  DEFAULT_OUTER_POINTS,
  OUTER_VIEWBOX_HEIGHT,
  OUTER_VIEWBOX_WIDTH,
} from "@/features/launcher/launcher-blob-shape";
import { cn } from "@/lib/utils";

interface HeroBlobShellProps {
  children: ReactNode;
  class_name?: string;
}

const OUTER_PATH = create_closed_spline_path(DEFAULT_OUTER_POINTS);
const OUTER_INNER_PATH_1 = create_closed_spline_path(
  create_inner_points(DEFAULT_OUTER_POINTS, 0.985, 0.982),
);
const OUTER_INNER_PATH_2 = create_closed_spline_path(
  create_inner_points(DEFAULT_OUTER_POINTS, 0.992, 0.99),
);

export function HeroBlobShell({ children, class_name }: HeroBlobShellProps) {
  const gradient_id = useId();
  const outer_edge_glow_gradient_id = useId();
  const outer_edge_glow_id = useId();
  const tint_gradient_id = useId();

  return (
    <div className={cn("relative w-full max-w-[404px] sm:max-w-[980px]", class_name)}>
      <div
        className="absolute inset-[-18%] z-0 translate-y-4 pointer-events-none blur-[28px] sm:translate-y-5"
        style={{ background: "var(--launcher-hero-aura)" }}
      />
      <div className="absolute inset-[-24%] z-0 origin-center translate-y-5 pointer-events-none scale-x-[1.7] scale-y-[0.96] sm:inset-[-20%] sm:translate-y-6 sm:scale-x-[1.4] sm:scale-y-[1.2] lg:scale-x-100 lg:scale-y-100">
        <svg
          aria-hidden="true"
          className="absolute inset-0 h-full w-full pointer-events-none"
          preserveAspectRatio="none"
          viewBox={`0 0 ${OUTER_VIEWBOX_WIDTH} ${OUTER_VIEWBOX_HEIGHT}`}
        >
          <defs>
            <linearGradient
              id={gradient_id}
              gradientUnits="userSpaceOnUse"
              x1="142"
              x2="900"
              y1="92"
              y2="640"
            >
              <stop offset="0%" style={{ stopColor: "var(--launcher-hero-stop-1)" }} />
              <stop offset="44%" style={{ stopColor: "var(--launcher-hero-stop-2)" }} />
              <stop offset="100%" style={{ stopColor: "var(--launcher-hero-stop-3)" }} />
            </linearGradient>
            <radialGradient id={tint_gradient_id} cx="20%" cy="18%" r="88%">
              <stop offset="0%" style={{ stopColor: "var(--launcher-hero-tint-1)" }} />
              <stop offset="42%" style={{ stopColor: "var(--launcher-hero-tint-2)" }} />
              <stop offset="74%" style={{ stopColor: "var(--launcher-hero-tint-3)" }} />
              <stop offset="100%" style={{ stopColor: "var(--launcher-hero-tint-4)" }} />
            </radialGradient>
            <linearGradient
              id={outer_edge_glow_gradient_id}
              gradientUnits="userSpaceOnUse"
              x1="176"
              x2="888"
              y1="74"
              y2="674"
            >
              <stop offset="0%" stopColor="rgba(255,255,255,0.74)" />
              <stop offset="34%" stopColor="rgba(255,255,255,0.28)" />
              <stop offset="74%" stopColor="rgba(211,224,248,0.18)" />
              <stop offset="100%" stopColor="rgba(255,255,255,0.12)" />
            </linearGradient>
            <filter id={outer_edge_glow_id} x="-20%" y="-80%" width="140%" height="260%">
              <feGaussianBlur stdDeviation="5.2" />
            </filter>
          </defs>

          <path
            d={OUTER_PATH}
            fill="none"
            filter={`url(#${outer_edge_glow_id})`}
            opacity={0.76}
            stroke={`url(#${outer_edge_glow_gradient_id})`}
            strokeWidth="16"
          />
          <path
            d={OUTER_INNER_PATH_1}
            fill="none"
            filter={`url(#${outer_edge_glow_id})`}
            opacity={0.60}
            stroke="rgba(255,255,255,0.18)"
            strokeWidth="12"
          />
          <path
            d={OUTER_PATH}
            fill={`url(#${gradient_id})`}
            opacity="0.96"
            stroke="var(--launcher-hero-stroke)"
            strokeWidth="1.8"
          />
          <path
            d={OUTER_PATH}
            fill={`url(#${tint_gradient_id})`}
            style={{ mixBlendMode: "soft-light" }}
          />
          <path
            d={OUTER_INNER_PATH_2}
            fill="var(--launcher-hero-inner-fill)"
            opacity="0.92"
            stroke="var(--launcher-hero-inner-stroke)"
            strokeWidth="3.2"
          />
        </svg>
      </div>

      <div className="relative z-10 px-5 py-11 text-center sm:px-14 sm:py-12 lg:px-18 lg:py-16">
        {children}
      </div>
    </div>
  );
}
