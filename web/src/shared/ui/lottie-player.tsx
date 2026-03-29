"use client";

import { useEffect, useState } from "react";
import type { DotLottie } from "@lottiefiles/dotlottie-react";
import { DotLottieReact } from "@lottiefiles/dotlottie-react";
import { CSSProperties } from "react";

interface LottiePlayerProps {
  src: string;
  class_name?: string;
  inline_style?: CSSProperties;
}

export function LottiePlayer({ src, class_name, inline_style }: LottiePlayerProps) {
  const [dotLottieInstance, setDotLottieInstance] = useState<DotLottie | null>(null);

  useEffect(() => {
    if (dotLottieInstance) {
      dotLottieInstance.play();
    }
  }, [dotLottieInstance]);

  return (
    <div className={class_name} style={inline_style}>
      <DotLottieReact
        autoplay
        dotLottieRefCallback={setDotLottieInstance}
        loop
        src={src}
      />
    </div>
  );
}
