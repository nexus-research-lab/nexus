// INPUT: 已选 Agent 名称与包含提及、邮件、相似名称的输入原文。
// OUTPUT: 提及只命中完整目标，镜像不改变任何原文字符。
// POS: Composer 提及投影回归。
import { describe, expect, it } from "vitest";
import { splitComposerMentions } from "./use-composer-mention";

describe("composer mention presentation", () => {
  it("preserves text while matching complete selected names, longest first", () => {
    const input = "/goal @Lucy @Lucy Liu，\n@C++ @Lucy。 @LucyX user@Lucy @Unknown ";
    const segments = splitComposerMentions(input, ["Lucy", "Lucy Liu", "C++", "Lucy"]);
    expect(segments.map((segment) => segment.text).join("")).toBe(input);
    expect(segments.filter((segment) => segment.mentioned).map((segment) => segment.text))
      .toEqual(["@Lucy", "@Lucy Liu", "@C++", "@Lucy"]);
  });

  it("leaves unselected and partially deleted mentions undecorated", () => {
    expect(splitComposerMentions("@Lucy", [])).toEqual([{ text: "@Lucy", mentioned: false }]);
    expect(splitComposerMentions("@Luc", ["Lucy"]).some((segment) => segment.mentioned)).toBe(false);
  });
});
