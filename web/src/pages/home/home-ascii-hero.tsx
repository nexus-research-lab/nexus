"use client";

import { useEffect, useRef } from "react";

import { usePrefersReducedMotion } from "@/hooks/use-prefers-reduced-motion";

const ASCII_CHARS = ".:+-=*#@&~<>{}[]|/\\";
const MOBILE_ASCII_CHARS = "01";
const HERO_LABEL = "nexus";
const HERO_BG = "#1e2124";
const HERO_BORDER = "#2e3138";
const HERO_INK = "#39fca8";        // bright green — nexus text
const CLOCK_INK = "#39fca8";      // same hue, lower alpha for clock

interface HomeAsciiHeroProps {
  agent_count: number;
  room_count: number;
}

interface AsciiParticle {
  x: number; y: number;
  tx: number; ty: number;
  vx: number; vy: number;
  char: string;
  alpha: number;
  target_alpha: number;
  is_text: boolean;
  phase: number;
  delay: number;
}

function pick(charset: string) {
  return charset[Math.floor(Math.random() * charset.length)] ?? ".";
}

function pad2(n: number) {
  return n.toString().padStart(2, "0");
}

export function HomeAsciiHero({ agent_count, room_count }: HomeAsciiHeroProps) {
  const section_ref = useRef<HTMLDivElement | null>(null);
  const canvas_ref = useRef<HTMLCanvasElement | null>(null);
  const prefers_reduced_motion = usePrefersReducedMotion();

  useEffect(() => {
    const section = section_ref.current;
    const canvas = canvas_ref.current;
    if (!section || !canvas || prefers_reduced_motion) {
      return;
    }

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const mobile_q = window.matchMedia("(max-width: 600px)");

    let particles: AsciiParticle[] = [];
    let W = 0, H = 0;
    let glyph_size = 6;
    let glyph_font = "";       // cached — avoid re-parsing font every frame
    let ir = 100, ir_sq = ir * ir, iforce = 3;
    let raf = 0;
    let px = -9999, py = -9999;
    let dead = false;
    let is_mobile = false;
    // clock layout — computed once on resize, not every frame
    let clock_pad_x = 22, clock_pad_y = 18;
    let clock_big_size = 28, clock_sm_size = 13, clock_meta_size = 10;
    let clock_font_big = "", clock_font_sm = "", clock_font_meta = "";
    let clock_hm_w = 0; // width of "HH:MM" — updated each second

    // ── clock state (updated every second) ────────────────────────────────────
    let clock_hh = "", clock_mm = "", clock_ss = "";
    let clock_timer = 0;

    function tick_clock() {
      const now = new Date();
      clock_hh = pad2(now.getHours());
      clock_mm = pad2(now.getMinutes());
      clock_ss = pad2(now.getSeconds());
      // Re-measure HH:MM width once per second (font must be set first; done after resize)
      if (clock_font_big) {
        ctx!.font = clock_font_big;
        clock_hm_w = ctx!.measureText(`${clock_hh}:${clock_mm}`).width;
      }
    }
    tick_clock();
    clock_timer = window.setInterval(tick_clock, 1000);

    // ── resize ────────────────────────────────────────────────────────────────
    function resize(w: number, h: number) {
      W = Math.max(w, 280);
      H = Math.max(h, 80);
      canvas!.width = Math.floor(W * dpr);
      canvas!.height = Math.floor(H * dpr);
      canvas!.style.width = `${W}px`;
      canvas!.style.height = `${H}px`;
      ctx!.setTransform(dpr, 0, 0, dpr, 0, 0);

      // Pre-compute clock layout so draw loop reads constants, not expressions
      clock_pad_x = is_mobile ? 14 : 22;
      clock_pad_y = is_mobile ? 12 : 18;
      clock_big_size = Math.round(Math.min(W * 0.072, H * 0.20, 56));
      clock_sm_size = Math.round(clock_big_size * 0.46);
      clock_meta_size = Math.round(Math.min(W * 0.018, 11));
      clock_font_big = `200 ${clock_big_size}px "IBM Plex Mono", monospace`;
      clock_font_sm = `200 ${clock_sm_size}px "IBM Plex Mono", monospace`;
      clock_font_meta = `400 ${clock_meta_size}px "IBM Plex Mono", monospace`;
      // Measure HH:MM width with new font
      ctx!.font = clock_font_big;
      clock_hm_w = ctx!.measureText(`${clock_hh}:${clock_mm}`).width;
    }

    // ── pointer ───────────────────────────────────────────────────────────────
    function set_ptr(client_x: number, client_y: number) {
      const b = canvas!.getBoundingClientRect();
      px = client_x - b.left; py = client_y - b.top;
    }
    const clear_ptr = () => { px = -9999; py = -9999; };
    const on_mouse = (e: MouseEvent) => set_ptr(e.clientX, e.clientY);
    const on_touch = (e: TouchEvent) => {
      const t = e.touches[0]; if (t) set_ptr(t.clientX, t.clientY);
    };

    // ── scene init ────────────────────────────────────────────────────────────
    const init = async () => {
      if (raf !== 0) { cancelAnimationFrame(raf); raf = 0; }

      // Capture refs — canvas/ctx are guaranteed non-null at this point (checked above).
      const cv = canvas!;
      const cx = ctx!;

      is_mobile = mobile_q.matches;
      const charset = is_mobile ? MOBILE_ASCII_CHARS : ASCII_CHARS;
      const step = is_mobile ? 2 : 4;
      glyph_size = is_mobile ? 3 : 6;
      glyph_font = `500 ${glyph_size}px "IBM Plex Mono", monospace`;
      ir = is_mobile ? 50 : 110;
      ir_sq = ir * ir;
      iforce = is_mobile ? 5 : 3.5;

      resize(section.clientWidth, section.clientHeight);

      if ("fonts" in document) {
        try { await document.fonts.ready; } catch { /* */ }
      }

      // ── sample hero text into particles ────────────────────────────────────
      const mc = document.createElement("canvas").getContext("2d")!;
      mc.font = '600 80px "IBM Plex Mono", monospace';
      const mw = mc.measureText(HERO_LABEL).width || W;

      // fit font to ~58% of height, max 92% of width
      const fs_w = Math.floor((80 * W) / mw * 0.92);
      const fs_h = Math.floor(H * 0.58);
      const fs = Math.min(fs_w, fs_h);
      const font = `600 ${fs}px "IBM Plex Mono", monospace`;

      const off = document.createElement("canvas");
      off.width = W; off.height = H;
      const oc = off.getContext("2d")!;
      oc.font = font;
      const tw = oc.measureText(HERO_LABEL).width;
      oc.fillStyle = "#fff";
      oc.textBaseline = "middle";
      // vertical center slightly above midpoint for visual balance
      oc.fillText(HERO_LABEL, Math.max(0, (W - tw) / 2), H * 0.46);

      const img = oc.getImageData(0, 0, W, H);
      const next: AsciiParticle[] = [];

      for (let y = 0; y < H; y += step) {
        for (let x = 0; x < W; x += step) {
          if (img.data[(y * W + x) * 4 + 3] <= 80) continue;
          next.push({
            x: x + (Math.random() - 0.5) * W * 0.45,
            y: y + (Math.random() - 0.5) * H * 2.2,
            tx: x, ty: y,
            vx: 0, vy: 0,
            char: pick(charset),
            alpha: 0,
            target_alpha: is_mobile ? 0.95 : 0.82 + Math.random() * 0.18,
            is_text: true,
            phase: Math.random() * Math.PI * 2,
            delay: (x / W) + Math.random() * 0.15,
          });
        }
      }

      // ambient noise particles
      const ambient = Math.max(40, Math.floor(next.length * 0.12));
      for (let i = 0; i < ambient; i++) {
        const x = Math.random() * W;
        const y = Math.random() * H;
        next.push({
          x, y, tx: x, ty: y,
          vx: (Math.random() - 0.5) * 0.12,
          vy: (Math.random() - 0.5) * 0.12,
          char: pick(charset),
          alpha: 0,
          target_alpha: 0.03 + Math.random() * 0.06,
          is_text: false,
          phase: Math.random() * Math.PI * 2,
          delay: Math.random() * 0.5,
        });
      }

      particles = next;
      const t0 = performance.now();

      // ── draw loop ──────────────────────────────────────────────────────────
      const has_ptr = () => px > -9000;

      const draw = (now: number) => {
        if (dead) { raf = 0; return; }

        const elapsed = (now - t0) / 1000;
        const ptr = has_ptr();
        ctx.clearRect(0, 0, W, H);

        // Set shared state once per frame
        ctx.font = glyph_font;
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillStyle = HERO_INK;

        let last_alpha = -1; // track last globalAlpha to skip redundant sets

        for (const p of particles) {
          const prog = Math.max(0, elapsed - p.delay);

          if (p.is_text && prog < 0.01) {
            if (last_alpha !== 0.02) { ctx.globalAlpha = 0.02; last_alpha = 0.02; }
            ctx.fillText(p.char, p.x, p.y);
            continue;
          }

          p.vx += (p.tx - p.x) * 0.038;
          p.vy += (p.ty - p.y) * 0.038;

          // Use squared distance to avoid Math.hypot (sqrt) when pointer is far
          if (ptr) {
            const ddx = p.x - px, ddy = p.y - py;
            const dist_sq = ddx * ddx + ddy * ddy;
            if (dist_sq < ir_sq && dist_sq > 0) {
              const dist = Math.sqrt(dist_sq);
              const f = ((1 - dist / ir) ** 2) * iforce;
              p.vx += (ddx / dist) * f;
              p.vy += (ddy / dist) * f;
            }
          }

          p.vx *= 0.87; p.vy *= 0.87;
          p.x += p.vx; p.y += p.vy;
          p.alpha += (p.target_alpha - p.alpha) * 0.04;

          if (p.is_text) {
            p.alpha = p.target_alpha + Math.sin(elapsed * 0.7 + p.phase) * 0.07;
            if (prog < 0.9 || Math.random() < 0.0006) p.char = pick(charset);
          } else {
            p.tx += (Math.random() - 0.5) * 0.18;
            p.ty += (Math.random() - 0.5) * 0.18;
            if (p.x < -20) p.x = p.tx = W + 10;
            if (p.x > W + 20) p.x = p.tx = -10;
            if (p.y < -20) p.y = p.ty = H + 10;
            if (p.y > H + 20) p.y = p.ty = -10;
            if (Math.random() < 0.003) p.char = pick(charset);
          }

          const a = Math.max(0, p.alpha);
          if (a !== last_alpha) { ctx.globalAlpha = a; last_alpha = a; }
          ctx.fillText(p.char, p.x, p.y);
        }

        // ── clock overlay ──────────────────────────────────────────────────
        ctx.textAlign = "left";
        ctx.textBaseline = "bottom";
        ctx.fillStyle = CLOCK_INK;

        const clock_y = H - clock_pad_y - clock_big_size * 0.28;

        ctx.font = clock_font_big;
        ctx.globalAlpha = 0.82; last_alpha = 0.82;
        ctx.fillText(`${clock_hh}:${clock_mm}`, clock_pad_x, clock_y);

        ctx.font = clock_font_sm;
        ctx.globalAlpha = 0.38; last_alpha = 0.38;
        ctx.fillText(`:${clock_ss}`, clock_pad_x + clock_hm_w + 2, clock_y + (clock_big_size - clock_sm_size) * 0.82);

        ctx.font = clock_font_meta;
        ctx.globalAlpha = 0.28;
        ctx.fillText(
          `AGENTS ${agent_count}  ·  ROOMS ${room_count}`,
          clock_pad_x,
          H - clock_pad_y - clock_big_size - 6,
        );

        ctx.globalAlpha = 1;
        raf = requestAnimationFrame(draw);
      };

      raf = requestAnimationFrame(draw);
    };

    const ro = new ResizeObserver(() => { void init(); });
    ro.observe(section);

    canvas.addEventListener("mousemove", on_mouse, { passive: true });
    canvas.addEventListener("mouseleave", clear_ptr);
    canvas.addEventListener("touchstart", on_touch, { passive: true });
    canvas.addEventListener("touchmove", on_touch, { passive: true });
    canvas.addEventListener("touchend", clear_ptr);

    void init();

    return () => {
      dead = true;
      clearInterval(clock_timer);
      if (raf !== 0) cancelAnimationFrame(raf);
      ro.disconnect();
      canvas.removeEventListener("mousemove", on_mouse);
      canvas.removeEventListener("mouseleave", clear_ptr);
      canvas.removeEventListener("touchstart", on_touch);
      canvas.removeEventListener("touchmove", on_touch);
      canvas.removeEventListener("touchend", clear_ptr);
    };
  }, [prefers_reduced_motion, agent_count, room_count]);

  return (
    <div
      ref={section_ref}
      className="relative h-full w-full overflow-hidden rounded-[28px] border"
      style={{ background: HERO_BG, borderColor: HERO_BORDER }}
    >
      {/* subtle radial glow */}
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_60%_50%_at_50%_50%,rgba(57,252,168,0.05),transparent)]" />

      <h2 className="sr-only">{HERO_LABEL}</h2>

      {prefers_reduced_motion ? (
        <div
          className="absolute inset-0 flex items-center justify-center font-mono text-[clamp(3rem,11vw,6.8rem)] font-light italic leading-none"
          style={{ color: HERO_INK }}
        >
          {HERO_LABEL}
        </div>
      ) : (
        <canvas
          ref={canvas_ref}
          className="absolute inset-0 block cursor-crosshair"
        />
      )}
    </div>
  );
}
