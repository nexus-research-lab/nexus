// INPUT: Decorative animation source and outer layout classes.
// OUTPUT: A looping decoration, or its static first frame when reduced motion is preferred.
// POS: Shared Lottie lifecycle owner; no business state or accessible content.
"use client";

import { DotLottieReact } from "@lottiefiles/dotlottie-react";
import { usePrefersReducedMotion } from "@/shared/lib/react/use-prefers-reduced-motion";

interface LottiePlayerProps {
  src: string;
  className?: string;
}

export function LottiePlayer({ src, className }: LottiePlayerProps) {
  const reducedMotion = usePrefersReducedMotion();

  return (
    <div
      aria-hidden="true"
      className={className}
    >
      <DotLottieReact
        autoplay={!reducedMotion}
        backgroundColor="transparent"
        className="block h-full w-full"
        // The player reads autoplay at load time, so preference changes replace its lifecycle.
        key={reducedMotion ? "still" : "playing"}
        loop={!reducedMotion}
        src={src}
      />
    </div>
  );
}
