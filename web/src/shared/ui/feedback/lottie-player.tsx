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

export function LottiePlayer({ src, class_name: className, inline_style: inlineStyle }: LottiePlayerProps) {
  const [dotLottieInstance, setDotLottieInstance] = useState<DotLottie | null>(null);

  useEffect(() => {
    if (dotLottieInstance) {
      dotLottieInstance.play();
    }
  }, [dotLottieInstance]);

  return (
    <div
      className={className}
      style={inlineStyle}
    >
      <DotLottieReact
        autoplay
        backgroundColor="transparent"
        className="block h-full w-full"
        dotLottieRefCallback={setDotLottieInstance}
        loop
        src={src}
      />
    </div>
  );
}
