// INPUT: 思考/等待状态、自然语言文案与固定活动槽位选项。
// OUTPUT: 验证静态文案、共享 LoadingOrb 所有权与稳定占位。
// POS: DM/Room 公用消息活动行回归测试。

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { MessageActivityStatus } from "./message-activity-status";

describe("MessageActivityStatus", () => {
  it("keeps copy and geometry still while shared LoadingOrb owns motion", () => {
    const { container, rerender } = render(
      <MessageActivityStatus
        label="正在思考"
        stableSlot
        state="thinking"
      />,
    );

    expect(container.querySelector("[data-loading-orb='active']")).toBeTruthy();
    expect(container.querySelector(".message-activity-label-flow")).toBeNull();
    expect(container.querySelector(".message-activity-spinner-track")).toBeNull();

    rerender(
      <MessageActivityStatus
        label="等待确认"
        stableSlot
        state="waiting_permission"
      />,
    );
    expect(container.querySelector("[data-loading-orb]")).toBeNull();

  });

});
