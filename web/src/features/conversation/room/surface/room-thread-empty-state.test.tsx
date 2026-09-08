// INPUT: Thread 已确认的加载/空状态与双语内容。
// OUTPUT: 共享无边框状态面、唯一播报与加载装饰消失的回归。
// POS: Thread 空态适配测试，不发起读取或建立交互动作。

import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { RoomThreadEmptyState } from "./room-thread-empty-state";

it.each(["en", "zh"] as const)("announces %s loading and empty details without a second visual anchor", (locale) => {
  const view = (isLoading: boolean) => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <RoomThreadEmptyState isLoading={isLoading} />
  </I18N_CONTEXT.Provider>;
  const rendered = render(view(true));
  const status = screen.getByRole("status");
  expect(status.getAttribute("aria-live")).toBe("polite");
  expect(status.getAttribute("aria-busy")).toBe("true");
  expect(status.textContent).toBe(MESSAGES[locale]["room.thread_waiting"]);
  expect(status.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
  expect(status.querySelector("svg")?.getAttribute("class")).toContain("motion-reduce:animate-none");
  expect(status.className).toContain("min-h-32");
  expect(status.className).not.toContain("border");
  rendered.rerender(view(false));
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(status.getAttribute("aria-busy")).toBe("false");
  expect(status.textContent).toBe(MESSAGES[locale]["room.thread_empty"]);
  expect(status.querySelector("svg")).toBeNull();
  expect(screen.queryByRole("button")).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
});
