// INPUT: Workspace 的用户可见名称和相对文件路径。
// OUTPUT: 证明显示标签不泄漏物理目录或 Agent ID，并生成稳定的相对位置。
// POS: Workspace 路径显示合同测试；文件事务与导航状态由各自控制器负责。

import { describe, expect, it } from "vitest";

import {
  getWorkspaceFileLocationSegments,
  getWorkspaceRootLabel,
} from "./workspace-path-model";

describe("workspace path labels", () => {
  it("uses the Agent display name instead of a physical workspace directory", () => {
    expect(getWorkspaceRootLabel("  Nexus 助手  ", "nexus", "工作区"))
      .toBe("Nexus 助手");
  });

  it("falls back through the Agent name to the localized workspace label", () => {
    expect(getWorkspaceRootLabel(" ", "nexus", "工作区")).toBe("nexus");
    expect(getWorkspaceRootLabel(null, " ", "工作区")).toBe("工作区");
  });

  it("renders file locations relative to the named workspace root", () => {
    expect(getWorkspaceFileLocationSegments("MEMORY.md", "Nexus 助手"))
      .toEqual(["Nexus 助手"]);
    expect(getWorkspaceFileLocationSegments("memory/archive/MEMORY.md", "Nexus 助手"))
      .toEqual(["Nexus 助手", "memory", "archive"]);
  });
});
