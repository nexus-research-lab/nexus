import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readSource = (relativePath) => readFile(
  new URL(`../${relativePath}`, import.meta.url),
  "utf8",
);

test("interactive list rows ignore keyboard events from nested controls", async () => {
  const listRow = await readSource("src/shared/ui/list/list-row.tsx");

  assert.match(listRow, /event\.target !== event\.currentTarget/);
  assert.match(listRow, /event\.key === "Enter" \|\| event\.key === " "/);
});
