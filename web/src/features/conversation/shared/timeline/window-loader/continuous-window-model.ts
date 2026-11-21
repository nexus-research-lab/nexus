/**
 * INPUT: 滚动视口几何、可见 round 在驻留窗口内的下标、窗口断点与各边界当前是否可请求。
 * OUTPUT: 下一次应向哪一端扩展连续窗口，以及超出内存预算时应让出的窗口端。
 * POS: 连续历史窗口的纯决策模型；不读取 DOM、不发请求、不持有 React 状态。
 */
export type WindowEdgeLoad = "older" | "newer" | "latest";

/** 距窗口起点不足一个视口时预取上一页，保证连续上滚不会撞到未加载边界。 */
const OLDER_EDGE_VIEWPORTS = 1;
/** 断点距最后一个可见 round 不超过该数量时向后扩展。 */
const NEWER_EDGE_ROUND_MARGIN = 2;
/** 边界请求失败或没有进展后，同一端在该时间内不再自动重试。 */
export const WINDOW_EDGE_COOLDOWN_MS = 1_500;

export interface WindowEdgeContext {
  clientHeight: number;
  /** 最近一次用户滚动方向；两端同时满足条件时优先沿阅读方向扩展。 */
  direction: "up" | "down";
  /** 首个/最后一个可见 round 在驻留窗口内的下标；视口内没有 round 时为 -1。 */
  firstVisibleIndex: number;
  lastVisibleIndex: number;
  /** 断点前最后一个驻留 round 的下标；没有断点时为 null，断点不可见时为 -1。 */
  gapIndex: number | null;
  newerUnavailable: boolean;
  olderUnavailable: boolean;
  scrollHeight: number;
  scrollTop: number;
}

export function resolveWindowEdgeLoad(
  context: WindowEdgeContext,
): WindowEdgeLoad | null {
  const newer = context.newerUnavailable ? null : resolveNewerEdgeLoad(context);
  if (newer === "latest") {
    return newer;
  }
  const older = (
    !context.olderUnavailable
    && context.scrollTop <= context.clientHeight * OLDER_EDGE_VIEWPORTS
  ) ? "older" : null;
  if (older && newer) {
    return context.direction === "down" ? newer : older;
  }
  return older ?? newer;
}

function resolveNewerEdgeLoad(
  context: WindowEdgeContext,
): Exclude<WindowEdgeLoad, "older"> | null {
  const { gapIndex } = context;
  if (gapIndex === null) {
    return null;
  }
  if (gapIndex < 0 || context.lastVisibleIndex < 0) {
    // 断点 round 没有可见节点时退回像素判定：接近内容底部即向后扩展。
    const distanceToBottom = (
      context.scrollHeight - context.scrollTop - context.clientHeight
    );
    return distanceToBottom <= context.clientHeight ? "newer" : null;
  }
  if (context.firstVisibleIndex > gapIndex) {
    // 读者已越过断点进入实时尾部：旧窗口不再与阅读位置相邻，重新接上最新一页。
    return "latest";
  }
  return gapIndex - context.lastVisibleIndex <= NEWER_EDGE_ROUND_MARGIN
    ? "newer"
    : null;
}

/**
 * 实时写入超出预算时让出离阅读视口更远的一端；FOLLOW 中最新一端就是阅读位置，
 * 即使首屏定位尚未完成也不能被淘汰。
 */
export function resolveWindowEvictionEdge(
  viewport: Pick<WindowEdgeContext, "clientHeight" | "scrollHeight" | "scrollTop"> & {
    followingLatest: boolean;
  },
): "head" | "tail" {
  if (viewport.followingLatest) {
    return "head";
  }
  const viewportCenter = viewport.scrollTop + viewport.clientHeight / 2;
  return viewportCenter < viewport.scrollHeight / 2 ? "tail" : "head";
}
