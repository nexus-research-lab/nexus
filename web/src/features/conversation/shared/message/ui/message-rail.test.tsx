// INPUT: 流式明细内容增长、程序化贴底产生的异步 scroll 事件与用户上滚。
// OUTPUT: 证明明细区只在用户向上滚动时退出跟随，不被自身贴底的滞后 scroll 事件误判。
// POS: MessageDetailScroll 跟随行为测试；jsdom 无布局，尺寸与 ResizeObserver 由测试显式驱动。

import { act, fireEvent, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MessageDetailScroll } from "./message-rail";

const CLIENT_HEIGHT = 280;
let resizeCallbacks: Array<() => void> = [];

class ResizeObserverStub {
  private readonly callback: () => void;
  constructor(callback: () => void) {
    this.callback = callback;
    resizeCallbacks.push(callback);
  }
  observe() {}
  disconnect() {
    resizeCallbacks = resizeCallbacks.filter((item) => item !== this.callback);
  }
}

function renderDetail() {
  const view = render(
    <MessageDetailScroll followContent>
      <p>thinking</p>
    </MessageDetailScroll>,
  );
  const element = view.container.querySelector<HTMLElement>("[data-message-detail-scroll]")!;
  let scrollHeight = CLIENT_HEIGHT;
  Object.defineProperty(element, "clientHeight", { configurable: true, get: () => CLIENT_HEIGHT });
  Object.defineProperty(element, "scrollHeight", { configurable: true, get: () => scrollHeight });
  return {
    element,
    // 流式内容提交：只改变布局，ResizeObserver 回调在之后的帧才执行。
    grow(delta: number) {
      scrollHeight += delta;
    },
    observeResize() {
      act(() => resizeCallbacks.forEach((callback) => callback()));
    },
    bottomTop: () => scrollHeight - CLIENT_HEIGHT,
  };
}

describe("MessageDetailScroll follow", () => {
  beforeEach(() => {
    vi.stubGlobal("ResizeObserver", ResizeObserverStub);
  });
  afterEach(() => {
    resizeCallbacks = [];
    vi.unstubAllGlobals();
  });

  it("stops following once the reader scrolls up, and resumes at the bottom", () => {
    const detail = renderDetail();
    detail.grow(400);
    detail.observeResize();

    detail.element.scrollTop = 100;
    fireEvent.scroll(detail.element);
    detail.grow(60);
    detail.observeResize();
    expect(detail.element.scrollTop).toBe(100);

    detail.element.scrollTop = detail.bottomTop();
    fireEvent.scroll(detail.element);
    detail.grow(60);
    detail.observeResize();
    expect(detail.element.scrollTop).toBe(detail.bottomTop());
  });
});
