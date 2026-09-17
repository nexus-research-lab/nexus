// INPUT: 各恢复阶段的 journal 快照与 owner scope。
// OUTPUT: 阶段不变量边界测试；default 阶段 unknown 写入必须携带偏好基线版本。
// POS: 恢复层阶段契约回归，不涉及网络或密钥。
import { afterEach, describe, expect, it } from "vitest";

import {
  writeProviderSetupJournal,
  type ProviderSetupJournal,
} from "./provider-setup-recovery";

function journal(overrides: Partial<ProviderSetupJournal>): ProviderSetupJournal {
  return {
    apiFormat: "chat_completions",
    baselineConfigurationVersion: null,
    configurationFingerprint: "a".repeat(64),
    configurationVersion: 2,
    credentialMode: "replace",
    model: "sample-model",
    outcome: "ready",
    ownerScope: "test-owner",
    preferencesBaselineVersion: null,
    presetKey: "custom",
    providerDisplayName: "Sample",
    providerKey: "custom-x",
    providerWasExisting: false,
    stage: "default",
    testBaselineAt: null,
    version: 1,
    ...overrides,
  };
}

afterEach(() => {
  window.localStorage.clear();
});

describe("provider setup journal stage invariants", () => {
  it("persists the default stage once the preferences baseline is captured", () => {
    expect(writeProviderSetupJournal(journal({
      outcome: "unknown",
      preferencesBaselineVersion: 7,
    }))).toBe(true);
  });

  it("rejects a default-stage unknown journal without a preferences baseline", () => {
    // ensureDefaultSelection 在写 unknown journal 前必须记录偏好基线版本；
    // 缺失时恢复层拒绝落盘，向导会在保存成功后永久卡在默认选择阶段。
    expect(writeProviderSetupJournal(journal({ outcome: "unknown" }))).toBe(false);
  });
});
