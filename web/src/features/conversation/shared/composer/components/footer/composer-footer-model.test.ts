// INPUT: Composer runtime/Goal/附件/错误组合与字符限制状态。
// OUTPUT: 验证状态优先级、语义 tone、加载阶段和字符风险投影。
// POS: Composer Footer 纯模型回归测试。

import { describe, expect, it } from "vitest";

import {
  type ComposerFooterStatusCopy,
  getCharacterCountTone,
  projectComposerFooterStatus,
} from "./composer-footer-model";

const copy: ComposerFooterStatusCopy = {
  compacting: "正在压缩",
  goalConfirming: "正在确认目标",
  goalCreating: "正在创建目标",
  preparingAttachments: "正在准备附件",
  replying: "正在回复",
  sending: "正在发送",
  stopHint: "[Esc 停止]",
};

describe("composer footer model", () => {
  it("keeps runtime activity ahead of preparation, Goal, and error states", () => {
    expect(projectComposerFooterStatus({
      activeError: "发送失败",
      copy,
      isGoalConfirming: true,
      isGoalCreating: true,
      isPreparingAttachments: true,
      runtimeActivity: "compacting",
    })).toEqual({
      hint: "[Esc 停止]",
      indicator: "active",
      kind: "activity",
      message: "正在压缩…",
      tone: "brand",
    });
  });

  it("projects pending and failure states without visual class names", () => {
    expect(projectComposerFooterStatus({
      activeError: null,
      copy,
      isGoalConfirming: true,
      isGoalCreating: true,
      isPreparingAttachments: false,
      runtimeActivity: null,
    })).toEqual({
      hint: null,
      indicator: "preparing",
      kind: "goal",
      message: "正在确认目标",
      tone: "brand",
    });
    expect(projectComposerFooterStatus({
      activeError: "发送失败",
      copy,
      isGoalConfirming: false,
      isGoalCreating: false,
      isPreparingAttachments: false,
      runtimeActivity: null,
    })?.tone).toBe("danger");
  });

  it("keeps character risk priority semantic", () => {
    expect(getCharacterCountTone({ isNearLimit: true, isOverLimit: true })).toBe("danger");
    expect(getCharacterCountTone({ isNearLimit: true, isOverLimit: false })).toBe("warning");
    expect(getCharacterCountTone({ isNearLimit: false, isOverLimit: false })).toBe("soft");
  });
});
