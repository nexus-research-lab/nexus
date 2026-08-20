/**
 * INPUT: iframe 当前高度、最新可信高度与 show_widget 是否终态。
 * OUTPUT: live 单调高度或需要终态合并提交的缩减高度。
 * POS: Generative UI 宿主测高纯策略；React 计时与 iframe 消息监听只消费结果。
 */
const MIN_HEIGHT = 180;
const MAX_HEIGHT = 4000;

export interface GenerativeUIHeightRevision {
  height: number;
  settle: boolean;
}

export function resolveGenerativeUIHeightRevision(
  currentHeight: number,
  reportedHeight: number,
  complete: boolean,
): GenerativeUIHeightRevision {
  const height = Math.ceil(Math.min(
    MAX_HEIGHT,
    Math.max(MIN_HEIGHT, reportedHeight),
  ));
  if (!complete && height < currentHeight) {
    return { height: currentHeight, settle: false };
  }
  return {
    height,
    settle: complete && height < currentHeight,
  };
}
