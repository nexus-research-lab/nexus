/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：stage-glow.ts
# @Date   ：2026-04-05 18:32
# @Author ：leemysw
# 2026-04-05 18:32   Create
# =====================================================
*/

import type { CSSProperties } from "react";

export type StageGlowTone = "green" | "lilac" | "mist" | "peach";

const STAGE_GLOW_BACKGROUND_MAP: Record<StageGlowTone, string> = {
  lilac: "var(--glow-lilac)",
  green: "var(--glow-green)",
  mist: "var(--glow-mist)",
  peach: "var(--glow-peach)",
};

export function get_stage_glow_style(tone: StageGlowTone): CSSProperties {
  return {
    background: STAGE_GLOW_BACKGROUND_MAP[tone],
  };
}
