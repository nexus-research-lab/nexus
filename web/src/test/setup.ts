// INPUT: 每个 Vitest jsdom 测试文件的完成生命周期。
// OUTPUT: 自动卸载 React 树并启用 React act 环境。
// POS: 组件测试公共环境；不提供业务 mock 或跨测试共享状态。

import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean })
  .IS_REACT_ACT_ENVIRONMENT = true;

class TestResizeObserver implements ResizeObserver {
  disconnect() {}

  observe() {}

  unobserve() {}
}

globalThis.ResizeObserver ??= TestResizeObserver;
globalThis.requestAnimationFrame ??= (callback) => window.setTimeout(
  () => callback(performance.now()),
  0,
);
globalThis.cancelAnimationFrame ??= (handle) => window.clearTimeout(handle);

afterEach(() => {
  cleanup();
});
