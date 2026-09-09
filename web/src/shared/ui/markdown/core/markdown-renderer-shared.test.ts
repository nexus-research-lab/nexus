// INPUT: Workspace references inside differently delimited code spans.
// OUTPUT: Normalization preserves code source rather than inserting nested backticks.
// POS: Shared Markdown preprocessing regression.
import { expect, it } from "vitest";
import { normalizeMarkdownContent } from "./markdown-renderer-shared";
it("preserves multi-backtick code spans and normalizes later plain paths", () => {
  const content = "``code ` report.md`` and report.md";
  expect(normalizeMarkdownContent(content, (path) => path === "report.md" ? path : null, () => undefined))
    .toBe("``code ` report.md`` and `report.md`");
});
