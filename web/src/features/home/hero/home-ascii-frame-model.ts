// INPUT: 当前帧时间、上一帧时间与场景活跃截止时间。
// OUTPUT: Home ASCII 场景的归一化运动步长与是否继续请求动画帧。
// POS: Canvas 动画时序与休眠的纯状态投影，不访问 DOM。

export const HOME_ASCII_REFERENCE_FRAME_MS = 1000 / 60;
export const HOME_ASCII_INTRO_WAKE_MS = 2400;
export const HOME_ASCII_POINTER_WAKE_MS = 800;

const HOME_ASCII_MAX_FRAME_DELTA_MS = HOME_ASCII_REFERENCE_FRAME_MS * 2;

interface HomeAsciiFrameInput {
  activeUntil: number;
  now: number;
  previousFrameAt: number;
}

interface HomeAsciiFrameProjection {
  motionScale: number;
  shouldContinue: boolean;
}

export function projectHomeAsciiFrame({
  activeUntil,
  now,
  previousFrameAt,
}: HomeAsciiFrameInput): HomeAsciiFrameProjection {
  const frameDelta = previousFrameAt === 0
    ? HOME_ASCII_REFERENCE_FRAME_MS
    : Math.min(
      Math.max(now - previousFrameAt, 0),
      HOME_ASCII_MAX_FRAME_DELTA_MS,
    );
  return {
    motionScale: frameDelta / HOME_ASCII_REFERENCE_FRAME_MS,
    shouldContinue: now < activeUntil,
  };
}
