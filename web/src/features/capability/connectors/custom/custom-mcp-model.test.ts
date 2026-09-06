// INPUT: 脱敏配置、显式本地草稿身份以及参数/秘密行修改。
// OUTPUT: 证明草稿可确定性重建，行身份不进入协议，空行和重复键仍按既有规则拒绝。
// POS: Custom MCP 纯模型回归；实际输入和保存动作由同目录 Dialog 测试覆盖。

import { describe, expect, it } from "vitest";

import type { CustomMCPServer } from "@/types/capability/connector";

import {
  buildCustomMCPServerInput,
  createCustomMCPArgumentDraft,
  createCustomMCPDraft,
  createCustomMCPSecretDraft,
  validateCustomMCPDraft,
} from "./custom-mcp-model";

const SERVER: CustomMCPServer = {
  configuration_state: "ready", connector_id: "custom-mcp:test", enabled: true,
  name: " example ", type: "stdio", command: " node ",
  args: ["same", "same", " "], env: { TOKEN: null, TAG: " value " },
};

describe("Custom MCP draft identity and protocol projection", () => {
  it("rebuilds the same draft from explicit identity and round-trips values without sending row IDs", () => {
    const draft = createCustomMCPDraft(SERVER, "dialog-a");
    expect(createCustomMCPDraft(SERVER, "dialog-a")).toEqual(draft);
    const ids = [...draft.args, ...draft.env].map((row) => row.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(validateCustomMCPDraft(draft)).toBeNull();
    expect(buildCustomMCPServerInput(draft)).toEqual({
      name: "example", type: "stdio", command: "node",
      args: ["same", "same", " "], env: { TAG: " value ", TOKEN: null },
    });
  });

  it("rejects an empty argument but preserves the existing whitespace argument semantics", () => {
    const draft = createCustomMCPDraft(SERVER, "dialog-a");
    draft.args.push(createCustomMCPArgumentDraft("new-argument"));
    expect(validateCustomMCPDraft(draft)).toBe("args");
    draft.args.at(-1)!.value = " ";
    expect(validateCustomMCPDraft(draft)).toBeNull();
  });

  it.each(["env", "headers"] as const)("validates %s keys independently from local identity and masked values", (kind) => {
    const draft = createCustomMCPDraft(SERVER, "dialog-a");
    if (kind === "headers") {
      draft.type = "http";
      draft.url = "https://example.com/mcp";
      draft.authType = "headers";
    }
    draft[kind] = [createCustomMCPSecretDraft("first", "TOKEN")];
    expect(validateCustomMCPDraft(draft)).toBe(kind);
    draft[kind][0].configured = true;
    expect(validateCustomMCPDraft(draft)).toBeNull();
    draft[kind].push(createCustomMCPSecretDraft("second", " TOKEN ", "replacement"));
    expect(validateCustomMCPDraft(draft)).toBe(kind);
    draft[kind].shift();
    expect(validateCustomMCPDraft(draft)).toBeNull();
    expect(buildCustomMCPServerInput(draft)[kind]).toEqual({ TOKEN: "replacement" });
  });
});
