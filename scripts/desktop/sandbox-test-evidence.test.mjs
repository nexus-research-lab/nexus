import assert from "node:assert/strict";
import test from "node:test";
import { requirePassedTests } from "./sandbox-test-evidence.mjs";

const events = (...values) => values.map((value) => JSON.stringify(value)).join("\n");
const required = ["TestRealBoundary"];

test("requires the named native test rather than a package pass", () => {
  assert.throws(() => requirePassedTests(events({ Action: "pass", Package: "executor" }), 0, required), /did not pass/);
});

test("rejects an opt-in test skipped by a missing binary", () => {
  assert.throws(() => requirePassedTests(events({ Action: "skip", Test: required[0] }, { Action: "pass", Package: "executor" }), 0, required), /skipped/);
});

test("accepts completed required tests and ignores unrelated output events", () => {
  const output = events({ Action: "output", Output: "diagnostic\n" }, { Action: "pass", Test: required[0] }, { Action: "pass", Package: "executor" });
  assert.deepEqual(requirePassedTests(output, 0, required), required);
});

test("rejects a process failure even if a test already passed", () => {
  assert.throws(() => requirePassedTests(events({ Action: "pass", Test: required[0] }), 1, required), /exited/);
});

test("does not hide a later failing subtest or malformed evidence", () => {
  assert.throws(() => requirePassedTests(events({ Action: "pass", Test: required[0] }, { Action: "fail", Test: "TestRealBoundary/child" }), 0, required), /failure/);
  assert.throws(() => requirePassedTests("not JSON", 0, required), SyntaxError);
});

test("a passing child alone cannot satisfy the parent test", () => {
  assert.throws(() => requirePassedTests(events({ Action: "pass", Test: "TestRealBoundary/child" }), 0, required), /did not pass/);
});
