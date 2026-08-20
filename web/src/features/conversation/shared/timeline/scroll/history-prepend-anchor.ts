interface HistoryPrependSnapshot {
  scrollHeight: number;
}

export class HistoryPrependAnchor {
  private snapshot: HistoryPrependSnapshot | null = null;

  prepare(container: HTMLDivElement): void {
    this.snapshot = {
      scrollHeight: container.scrollHeight,
    };
  }

  cancel(): void {
    this.snapshot = null;
  }

  restore(container: HTMLDivElement): number | null {
    const snapshot = this.snapshot;
    if (!snapshot) {
      return null;
    }

    this.snapshot = null;
    // 网络等待期间用户仍可能继续滚动；以提交时的实时位置叠加前插高度，
    // 不能把请求开始时的旧 scrollTop 写回来抢走用户手势。
    const nextScrollTop =
      container.scrollTop + container.scrollHeight - snapshot.scrollHeight;
    container.scrollTop = nextScrollTop;
    return container.scrollTop;
  }
}
