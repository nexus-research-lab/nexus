// INPUT: active/preparing 语义帧型。
// OUTPUT: 不依赖 JS 计时器或运行时样式注入的共享加载指示器。
// POS: 轻量循环加载动效唯一 owner；消费者不得传私有字符帧或动画周期。

"use client";

export type LoadingOrbVariant = "active" | "preparing";

const FRAME_DURATION_MS = 120;
const LOADING_ORB_FRAME_MAP: Readonly<Record<LoadingOrbVariant, readonly string[]>> = {
  active: ["✽", "✻", "✶", "✢", "·"],
  preparing: ["·", "◦", "•", "◦"],
};

interface KeyedFrame {
  char: string;
  key: string;
  position: number;
}

function getKeyedFrames(frames: readonly string[]): KeyedFrame[] {
  const seenCounts = new Map<string, number>();
  const keyedFrames: KeyedFrame[] = [];
  let position = 0;

  for (const char of frames) {
    const occurrence = seenCounts.get(char) ?? 0;
    seenCounts.set(char, occurrence + 1);
    keyedFrames.push({
      char,
      key: `${char}-${occurrence}`,
      position,
    });
    position += 1;
  }

  return keyedFrames;
}

export function LoadingOrb({
  variant = "active",
}: {
  variant?: LoadingOrbVariant;
}) {
  const frames = LOADING_ORB_FRAME_MAP[variant];
  const keyedFrames = getKeyedFrames(frames);

  return (
    <span
      aria-hidden="true"
      className="relative inline-block w-3 select-none text-center leading-none text-primary"
      data-loading-orb={variant}
    >
      {keyedFrames.map(({ char, key, position }) => (
        <span
          className={position === 0
            ? "ui-loading-orb-frame"
            : "ui-loading-orb-frame absolute inset-0"}
          data-frame-count={frames.length}
          key={key}
          style={{ animationDelay: `${position * FRAME_DURATION_MS}ms` }}
        >
          {char}
        </span>
      ))}
    </span>
  );
}
