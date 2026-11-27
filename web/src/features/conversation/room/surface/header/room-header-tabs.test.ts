// INPUT: 视图标签定义表与主智能体开关。
// OUTPUT: 证明主智能体主窗口只得到工作区与简介，普通 DM/Room 按内容可用性隐藏空的工作图/子智能体入口，已打开的面板保留入口。
// POS: 顶栏视图标签投影合同；Header 布局与图标呈现由共享 Surface Header 负责。

import { describe, expect, it } from "vitest";

import { buildRoomHeaderTabs } from "./room-header-tabs";

const t = ((key: string) => key) as Parameters<typeof buildRoomHeaderTabs>[0];

describe("buildRoomHeaderTabs", () => {
  it("keeps every view for ordinary conversations", () => {
    expect(buildRoomHeaderTabs(t).map((tab) => tab.key)).toEqual(["workgraph", "subagents", "workspace", "about"]);
  });

  it("keeps only workspace and about for the main agent window", () => {
    expect(buildRoomHeaderTabs(t, { mainAgent: true }).map((tab) => tab.key)).toEqual(["workspace", "about"]);
  });

  it("hides empty workgraph and subagent views unless their panel is open", () => {
    const availability = { subagents: false, workgraph: false };
    expect(buildRoomHeaderTabs(t, { availability }).map((tab) => tab.key)).toEqual(["workspace", "about"]);
    expect(buildRoomHeaderTabs(t, { activeTab: "subagents", availability }).map((tab) => tab.key))
      .toEqual(["subagents", "workspace", "about"]);
  });
});
