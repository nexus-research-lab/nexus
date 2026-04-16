/**
 * =====================================================
 * @File   : skills-helpers.ts
 * @Date   : 2026-04-16 13:35
 * @Author : leemysw
 * 2026-04-16 13:35   Create
 * =====================================================
 */

export function format_installs(n: number) {
  return n >= 1000 ? `${(n / 1000).toFixed(n >= 100000 ? 0 : 1)}K` : `${n}`;
}
