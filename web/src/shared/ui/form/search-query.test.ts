// INPUT: 空白、大小写、兼容字符、数字和缺失搜索字段。
// OUTPUT: 证明全站客户端搜索使用同一标准化与空查询语义。
// POS: Search query 纯函数测试；搜索输入 DOM 由 form-controls.test.tsx 覆盖。

import { describe, expect, it } from "vitest";

import {
  createUiSearchMatcher,
  matchesUiSearchFields,
  normalizeUiSearchText,
} from "./search-query";

describe("shared search query", () => {
  it("normalizes whitespace, case, and Unicode compatibility forms", () => {
    expect(normalizeUiSearchText("  ＮＥＸＵＳ Agent  ")).toBe("nexus agent");
  });

  it("lets an empty query pass every item", () => {
    const matcher = createUiSearchMatcher("   ");
    expect(matcher.empty).toBe(true);
    expect(matcher.matches([])).toBe(true);
  });

  it("matches declared fields without leaking missing values into text", () => {
    expect(matchesUiSearchFields("agent 42", [null, "Nexus Agent 42", 7])).toBe(true);
    expect(matchesUiSearchFields("undefined", [undefined, null])).toBe(false);
  });

  it("lets a business surface opt into prefix matching", () => {
    const matcher = createUiSearchMatcher("GO");

    expect(matcher.matches(["goal", "workspace"], "prefix")).toBe(true);
    expect(matcher.matches(["forego"], "prefix")).toBe(false);
    expect(matcher.matches(["forego"])).toBe(true);
  });
});
