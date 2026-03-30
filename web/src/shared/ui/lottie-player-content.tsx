"use client";

import { type CSSProperties, useEffect, useState } from "react";
import type { DotLottie } from "@lottiefiles/dotlottie-react";
import { DotLottieReact } from "@lottiefiles/dotlottie-react";

interface LottiePlayerContentProps {
  src: string;
  class_name?: string;
  inline_style?: CSSProperties;
}

export function LottiePlayerContent({ src, class_name, inline_style }: LottiePlayerContentProps) {
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
