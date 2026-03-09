"use client";

import { useEffect, useState } from "react";

// 菱形展开：['·', '◇', '◆', '◈', '❖']
// 十字扩展：['·', '+', '✚', '✛', '✜']
// 开花：['✾', '❀', '✿', '❁', '❃']
// 星辰：['✦', '✧', '✩', '✪', '✫']
// 雪花：['·', '❄', '❅', '❆', '❉']
// 螺旋：['◌', '⊚', '⊛', '⊝', '◎']

export function LoadingOrb({frames = ["✽", "✻", "✶", "✢", "·"]}: { frames?: string[] }) {
  const [frame, setFrame] = useState(0);

  useEffect(() => {
    const timer = setInterval(() => {
      setFrame((prev) => (prev + 1) % frames.length);
    }, 120);
    return () => clearInterval(timer);
  }, []);

  return (
    <span className="inline-block w-3 text-center text-primary leading-none select-none">
        {frames[frame]}
    </span>
  );
}