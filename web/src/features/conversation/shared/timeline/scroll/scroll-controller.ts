/**
 * INPUT: 会话滚动容器，以及声明了所有者身份的绝对/相对滚动写入。
 * OUTPUT: 对 scrollTop 的唯一实际写入，以及同一帧内出现多个所有者时的冲突记录。
 * POS: DM/Room/Thread 消息视口的滚动写入边界；FOLLOW、阅读锚点、导航、
 * Virtualizer 与局部展开都只能经由这里移动视口。
 */

/** 允许移动会话视口的所有者；每一帧只应有其中一个真正写入。 */
export type ConversationScrollWriter =
  /** FOLLOW 贴底与显式回到底部。 */
  | "follow"
  /** READING 中按稳定 round 身份恢复阅读锚点（含历史前插）。 */
  | "anchor"
  /** Composer、虚拟键盘或窗口造成的 viewport 尺寸变化。 */
  | "viewport"
  /** 会话切换时的起始位置。 */
  | "session"
  /** 轮次导航跳转。 */
  | "navigation"
  /** Virtualizer 的测高补偿、窗口拓扑补偿与自身定位。 */
  | "virtualizer"
  /** 静态 Feed 内局部内容展开收起。 */
  | "local";

export interface ConversationScrollWriteConflict {
  first: ConversationScrollWriter;
  second: ConversationScrollWriter;
}

interface ScrollWriteLedger {
  conflicts: ConversationScrollWriteConflict[];
  frame: number | null;
  writer: ConversationScrollWriter | null;
}

const ledgers = new WeakMap<Element, ScrollWriteLedger>();

function claimFrame(
  element: Element,
  writer: ConversationScrollWriter,
): void {
  // 帧边界取自容器所在窗口；脱离文档的容器没有帧，也就没有同帧竞争可记。
  const view = element.ownerDocument?.defaultView;
  if (!view) {
    return;
  }
  let ledger = ledgers.get(element);
  if (!ledger) {
    ledger = { conflicts: [], frame: null, writer: null };
    ledgers.set(element, ledger);
  }
  const current = ledger;
  if (current.writer !== null && current.writer !== writer) {
    const conflict = { first: current.writer, second: writer };
    current.conflicts.push(conflict);
    if (import.meta.env.DEV) {
      console.warn(
        "[conversation-scroll] 同一帧内出现多个 scrollTop 写入者",
        conflict,
      );
    }
  }
  current.writer = writer;
  current.frame ??= view.requestAnimationFrame(() => {
    current.frame = null;
    current.writer = null;
  });
}

/** 把视口移动到绝对位置，返回浏览器 clamp 之后的实际 scrollTop。 */
export function writeConversationScrollTop(
  element: Element,
  writer: ConversationScrollWriter,
  scrollTop: number,
): number {
  const previous = element.scrollTop;
  element.scrollTop = scrollTop;
  // 没有移动视口的写入（重申当前位置、被浏览器 clamp 掉）不构成所有权竞争。
  if (element.scrollTop !== previous) {
    claimFrame(element, writer);
  }
  return element.scrollTop;
}

/**
 * 按位移补偿视口。位移始终叠加到写入瞬间的实时 scrollTop，
 * 因此不会回写用户在异步等待或手势期间自己产生的滚动。
 */
export function shiftConversationScrollTop(
  element: Element,
  writer: ConversationScrollWriter,
  delta: number,
): number {
  return writeConversationScrollTop(
    element,
    writer,
    element.scrollTop + delta,
  );
}

/** 交给浏览器执行的定位（可带 smooth）；所有权同样计入当前帧。 */
export function scrollConversationElementTo(
  element: Element,
  writer: ConversationScrollWriter,
  options: { behavior?: ScrollBehavior; top: number },
): void {
  const previous = element.scrollTop;
  element.scrollTo(options);
  const moves = options.behavior === "smooth"
    ? Math.round(options.top) !== Math.round(previous)
    : element.scrollTop !== previous;
  if (moves) {
    claimFrame(element, writer);
  }
}

/** 该容器自挂载以来记录到的跨所有者同帧写入，供开发诊断与浏览器测试断言。 */
export function getConversationScrollWriteConflicts(
  element: Element,
): readonly ConversationScrollWriteConflict[] {
  return ledgers.get(element)?.conflicts ?? [];
}
