"use client";

import { useEffect, useState } from "react";
import Lottie from "lottie-react";
import type { DotLottie } from "@lottiefiles/dotlottie-react";
import { DotLottieReact } from "@lottiefiles/dotlottie-react";
import { CSSProperties } from "react";

interface LottiePlayerProps {
  src: string;
  class_name?: string;
  inline_style?: CSSProperties;
}

export function LottiePlayer({ src, class_name, inline_style }: LottiePlayerProps) {
  const [animationData, setAnimationData] = useState<object | null>(null);
  const [dotLottieInstance, setDotLottieInstance] = useState<DotLottie | null>(null);
  const isDotLottie = src.endsWith(".lottie");

  useEffect(() => {
    if (isDotLottie) {
      return;
    }

    const controller = new AbortController();

    fetch(src, { signal: controller.signal })
      .then((response) => response.json())
      .then((json) => setAnimationData(json))
      .catch((error: unknown) => {
        if ((error as { name?: string })?.name !== "AbortError") {
          console.error("加载首页动画失败", error);
        }
      });

    return () => controller.abort();
  }, [isDotLottie, src]);

  useEffect(() => {
    if (dotLottieInstance) {
      dotLottieInstance.play();
    }
  }, [dotLottieInstance]);

  if (isDotLottie) {
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

  if (!animationData) {
    return <div className={class_name} style={inline_style} />;
  }

  return (
    <Lottie
      animationData={animationData}
      autoplay
      className={class_name}
      loop
      rendererSettings={{
        preserveAspectRatio: "xMidYMid meet",
      }}
      style={inline_style}
    />
  );
}
