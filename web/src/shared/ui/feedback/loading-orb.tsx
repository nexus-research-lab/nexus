"use client";

// Pure CSS animation — no JS timer, no React re-renders, runs on compositor thread.
// Each frame is a <span> with opacity. Stepped animation cycles through them.

const DEFAULT_FRAMES = ["✽", "✻", "✶", "✢", "·"];
const FRAME_DURATION_MS = 120;

let injected = false;
function ensure_style(count: number, duration: number) {
  if (injected || typeof document === "undefined") return;
  injected = true;
  // step(1, end) keyframe: visible 1/count of the total cycle, then hidden.
  const style = document.createElement("style");
  style.textContent = `
@keyframes _nexus_orb {
  0%, ${(100 / count - 0.01).toFixed(2)}%  { opacity: 1; }
  ${(100 / count).toFixed(2)}%, 100%        { opacity: 0; }
}
.nexus-orb-frame {
  display: inline-block;
  animation: _nexus_orb ${(duration * count).toFixed(0)}ms steps(1) infinite;
}`;
  document.head.appendChild(style);
}

interface KeyedFrame {
  char: string;
  key: string;
  position: number;
}

function get_keyed_frames(frames: string[]): KeyedFrame[] {
  const seen_counts = new Map<string, number>();
  const keyed_frames: KeyedFrame[] = [];
  let position = 0;

  for (const char of frames) {
    const occurrence = seen_counts.get(char) ?? 0;
    seen_counts.set(char, occurrence + 1);
    keyed_frames.push({
      char,
      key: `${char}-${occurrence}`,
      position,
    });
    position += 1;
  }

  return keyed_frames;
}

export function LoadingOrb({ frames = DEFAULT_FRAMES }: { frames?: string[] }) {
  ensure_style(frames.length, FRAME_DURATION_MS);
  const total = frames.length * FRAME_DURATION_MS;
  const keyed_frames = get_keyed_frames(frames);

  return (
    <span className="relative inline-block w-3 select-none text-center leading-none text-primary" aria-hidden>
      {keyed_frames.map(({ char, key, position }) => (
        <span
          key={key}
          className={position === 0 ? "nexus-orb-frame" : "nexus-orb-frame absolute inset-0"}
          style={{
            animationDelay: `${position * FRAME_DURATION_MS}ms`,
            animationDuration: `${total}ms`,
            opacity: 0,
          }}
        >
          {char}
        </span>
      ))}
    </span>
  );
}
