"use client";

import { useEffect, useState } from "react";

export function LoadingOrb({ frames = ["✽", "✻", "✶", "✢", "·"] }: { frames?: string[] }) {
  const [frame, setFrame] = useState(0);

  useEffect(() => {
    const timer = setInterval(() => {
      setFrame((prev) => (prev + 1) % frames.length);
    }, 120);
    return () => clearInterval(timer);
  }, [frames.length]);

  return (
    <span className="inline-block w-3 select-none text-center leading-none text-primary">
      {frames[frame]}
    </span>
  );
}
