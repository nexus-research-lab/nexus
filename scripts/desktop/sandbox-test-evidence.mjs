// INPUT: Go test JSON output, process exit status and explicitly required tests.
// OUTPUT: Confirmed test passes, or an error for failed, skipped or missing evidence.
// POS: Desktop sandbox acceptance guard; package success alone is insufficient.

export function requirePassedTests(output, exitCode, requiredTests) {
  if (exitCode !== 0) throw new Error(`Go test exited with ${exitCode}`);
  const passed = new Set();
  for (const line of output.split("\n").filter((value) => value.trim())) {
    const event = JSON.parse(line);
    if (event.Action === "fail") {
      throw new Error(`Go test failure: ${event.Test ?? event.Package}`);
    }
    if (event.Action === "skip" && requiredTests.includes(event.Test)) {
      throw new Error(`Required test skipped: ${event.Test}`);
    }
    if (event.Action === "pass" && event.Test) passed.add(event.Test);
  }
  const missing = requiredTests.filter((name) => !passed.has(name));
  if (missing.length) throw new Error(`Required test did not pass: ${missing.join(", ")}`);
  return [...requiredTests];
}
