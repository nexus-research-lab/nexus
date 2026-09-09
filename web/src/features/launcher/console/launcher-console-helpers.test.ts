// INPUT: 空目录名称与双语上下文。
// OUTPUT: 本地化缺省名称保留精确资源身份，Token保留完整Agent名。
// POS: Launcher目录展示投影回归。
import { expect, it } from "vitest";
import { MESSAGES } from "@/shared/i18n/messages";
import { buildDecorativeTokens, buildLauncherMentionTargets, buildRecentLauncherEntries } from "./launcher-console-helpers";
it("localizes fallback directory text without changing identities", () => {
  const agents = [{ id: "a", name: "Nova Researcher" }];
  const rooms = [{ id: "r", room_type: "room" as const, name: "" }];
  const en = buildLauncherMentionTargets(agents, rooms, (key) => MESSAGES.en[key]);
  const zh = buildLauncherMentionTargets(agents, rooms, (key) => MESSAGES.zh[key]);
  expect(en[1].label).toBe("Unnamed room"); expect(zh[1].label).toBe("未命名群聊");
  expect(en.map((item) => item.id)).toEqual(zh.map((item) => item.id));
  const recent = buildRecentLauncherEntries([{ session_key: "s", room_type: "dm", title: "", last_activity: "2026-09-10", agent_id: "a" }], (key) => MESSAGES.en[key]);
  expect(recent[0].label).toBe("Untitled conversation"); expect(recent[0].agent_id).toBe("a");
  expect(buildDecorativeTokens(agents, rooms)[0].name).toBe("Nova Researcher");
});
