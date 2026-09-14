// INPUT: User-visible names and initials length.
// OUTPUT: Stable initials without splitting Unicode display characters.
// POS: Identity fallback regressions shared by avatars and Launcher tokens.

import { expect, it } from "vitest";

import { getInitials } from "./avatar";

it.each([
  ["Maya Chen", 2, "MC"],
  ["Nova", 2, "NO"],
  ["张小明", 2, "张小"],
  ["👩‍💻 Nova", 1, "👩‍💻"],
  ["👩‍💻Nova", 2, "👩‍💻N"],
  ["e\u0301mile", 1, "E\u0301"],
  ["ß", 1, "S"],
  ["  𠮷野 家  ", 2, "𠮷家"],
])("keeps complete initials for %s", (name, length, expected) => {
  expect(getInitials(name, "AG", length)).toBe(expected);
});

it("retains the caller's empty-name fallback", () => {
  expect(getInitials("   ")).toBe("AG");
  expect(getInitials(null, "Room")).toBe("Room");
});
