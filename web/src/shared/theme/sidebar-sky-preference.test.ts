// INPUT: 合法、已下线与损坏的侧栏天空偏好值。
// OUTPUT: 归一化后的偏好；旧变体与损坏值回到跟随时间。
// POS: 侧栏天空偏好状态源的回归测试。
import { expect, it } from "vitest";

import { normalizeSidebarSkyPreference } from "./sidebar-sky-preference";

it.each([
  ["auto", "auto"],
  ["off", "off"],
  ["dawn", "dawn"],
  ["day", "day"],
  ["night", "night"],
  ["meteor", "meteor"],
  ["aurora", "auto"],
  ["constellation", "auto"],
  [null, "auto"],
  [42, "auto"],
])("normalizes %s to %s", (value, expected) => {
  expect(normalizeSidebarSkyPreference(value)).toBe(expected);
});
