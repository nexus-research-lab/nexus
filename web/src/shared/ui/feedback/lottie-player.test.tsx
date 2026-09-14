// INPUT: Decorative source changes and live reduced-motion preferences.
// OUTPUT: Only the current player lifecycle receives playback settings; canvas remains decorative.
// POS: Offline React adapter contract; does not emulate WASM rendering or native playback.

import { useEffect } from "react";
import { render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { LottiePlayer } from "./lottie-player";

const playback = vi.hoisted(() => ({ reduced: false, destroyed: vi.fn() }));
vi.mock("@/shared/lib/react/use-prefers-reduced-motion", () => ({
  usePrefersReducedMotion: () => playback.reduced,
}));
vi.mock("@lottiefiles/dotlottie-react", () => ({
  DotLottieReact: ({ autoplay, loop, src }: { autoplay: boolean; loop: boolean; src: string }) => {
    useEffect(() => () => { playback.destroyed(); }, []);
    return <canvas data-testid="decoration" data-autoplay={autoplay} data-loop={loop} data-src={src} />;
  },
}));
beforeEach(() => { playback.reduced = false; playback.destroyed.mockClear(); });

it("starts a static, decorative player when reduced motion is enabled", () => {
  playback.reduced = true;
  render(<LottiePlayer src="sparkles.lottie" className="h-12 w-12" />);
  const canvas = screen.getByTestId("decoration");
  expect(canvas.dataset.autoplay).toBe("false");
  expect(canvas.dataset.loop).toBe("false");
  expect(canvas.closest('[aria-hidden="true"]')).toBeTruthy();
});

it("disposes the playing lifecycle before switching to static and can resume playback", () => {
  const { rerender, unmount } = render(<LottiePlayer src="sparkles.lottie" />);
  const playing = screen.getByTestId("decoration");
  expect(playing.dataset.autoplay).toBe("true");
  expect(playing.dataset.loop).toBe("true");
  playback.reduced = true;
  rerender(<LottiePlayer src="sparkles.lottie" />);
  expect(screen.getByTestId("decoration")).not.toBe(playing);
  expect(screen.getByTestId("decoration").dataset.autoplay).toBe("false");
  expect(playback.destroyed).toHaveBeenCalledTimes(1);
  playback.reduced = false;
  rerender(<LottiePlayer src="sparkles.lottie" />);
  expect(screen.getByTestId("decoration").dataset.autoplay).toBe("true");
  expect(playback.destroyed).toHaveBeenCalledTimes(2);
  unmount();
  expect(playback.destroyed).toHaveBeenCalledTimes(3);
});

it("forwards new sources without restarting the same motion lifecycle on layout changes", () => {
  const { rerender } = render(<LottiePlayer src="first.lottie" />);
  const canvas = screen.getByTestId("decoration");
  rerender(<LottiePlayer src="second.lottie" className="h-24 w-24" />);
  expect(screen.getByTestId("decoration")).toBe(canvas);
  expect(canvas.dataset.src).toBe("second.lottie");
  expect(playback.destroyed).not.toHaveBeenCalled();
});
