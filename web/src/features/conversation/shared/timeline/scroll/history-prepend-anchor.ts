interface HistoryPrependSnapshot {
  scrollHeight: number;
  scrollTop: number;
}

export class HistoryPrependAnchor {
  private snapshot: HistoryPrependSnapshot | null = null;

  prepare(container: HTMLDivElement): void {
    this.snapshot = {
      scrollHeight: container.scrollHeight,
      scrollTop: container.scrollTop,
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
    const nextScrollTop =
      snapshot.scrollTop + container.scrollHeight - snapshot.scrollHeight;
    container.scrollTop = nextScrollTop;
    return container.scrollTop;
  }
}
