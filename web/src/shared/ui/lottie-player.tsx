"use client";

import { CSSProperties, useEffect, useState } from "react";
import Lottie from "lottie-react";
import type { DotLottie } from "@lottiefiles/dotlottie-react";
import { DotLottieReact } from "@lottiefiles/dotlottie-react";

interface HomeLottieProps {
  src: string;
  class_name?: string;
  style?: CSSProperties;
}

export function LottiePlayer({ src, class_name, style }: HomeLottieProps) {
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
      <div className={class_name} style={style}>
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
    return <div className={class_name} style={style} />;
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
      style={style}
    />
  );
}
