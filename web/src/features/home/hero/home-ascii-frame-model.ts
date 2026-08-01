// INPUT: 当前帧时间、最近绘制时间与场景活跃截止时间。
// OUTPUT: Home ASCII 场景是否绘制当前帧、是否继续请求动画帧。
// POS: Canvas 动画节流与休眠的纯状态投影，不访问 DOM。

export const HOME_ASCII_ACTIVE_FRAME_INTERVAL_MS = 1000 / 30;
export const HOME_ASCII_INTRO_WAKE_MS = 2400;
export const HOME_ASCII_POINTER_WAKE_MS = 800;

interface HomeAsciiFrameInput {
  activeUntil: number;
  lastPaintAt: number;
  now: number;
}

interface HomeAsciiFrameProjection {
  shouldContinue: boolean;
  shouldPaint: boolean;
}

export function projectHomeAsciiFrame({
  activeUntil,
  lastPaintAt,
  now,
}: HomeAsciiFrameInput): HomeAsciiFrameProjection {
  return {
    shouldContinue: now < activeUntil,
    shouldPaint:
      lastPaintAt === 0 ||
      now - lastPaintAt >= HOME_ASCII_ACTIVE_FRAME_INTERVAL_MS,
  };
}
