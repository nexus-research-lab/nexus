export interface StageWindowDragOffset {
  x: number;
  y: number;
}

export interface StageWindowResizeSize {
  height: number;
  width: number;
}

const MAX_DESKTOP_DRAG_X = 520;
const MAX_DESKTOP_DRAG_Y = 300;
const MIN_DESKTOP_DRAG_X = -520;
const MIN_DESKTOP_DRAG_Y = -260;
const MAX_WINDOW_HEIGHT = 1000;
const MAX_WINDOW_WIDTH = 1600;
const MIN_WINDOW_HEIGHT = 220;
const MIN_WINDOW_WIDTH = 320;

export function normalize_stage_window_drag_offset(offset: StageWindowDragOffset): StageWindowDragOffset {
  return {
    x: clamp_window_drag_axis(offset.x, MIN_DESKTOP_DRAG_X, MAX_DESKTOP_DRAG_X),
    y: clamp_window_drag_axis(offset.y, MIN_DESKTOP_DRAG_Y, MAX_DESKTOP_DRAG_Y),
  };
}

export function is_meaningful_stage_window_drag(offset: StageWindowDragOffset): boolean {
  return Math.abs(offset.x) >= 2 || Math.abs(offset.y) >= 2;
}

export function normalize_stage_window_resize_size(size: StageWindowResizeSize): StageWindowResizeSize {
  return {
    height: clamp_window_drag_axis(size.height, MIN_WINDOW_HEIGHT, MAX_WINDOW_HEIGHT),
    width: clamp_window_drag_axis(size.width, MIN_WINDOW_WIDTH, MAX_WINDOW_WIDTH),
  };
}

function clamp_window_drag_axis(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.min(max, Math.max(min, Math.round(value)));
}
