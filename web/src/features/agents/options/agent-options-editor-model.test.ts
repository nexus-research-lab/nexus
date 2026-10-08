import { afterEach, expect, it, vi } from "vitest";
import { decodeAvatar } from "@/shared/lib/humation/avatar";
import { buildAgentOptionsCreateSource } from "./agent-options-editor-model";

afterEach(() => vi.unstubAllGlobals());

it("creates Agent drafts with a saved Humation identity", () => {
  const draft = buildAgentOptionsCreateSource({});
  expect(decodeAvatar(draft.initial.avatar)).not.toBeNull();
});

it("creates an Agent draft when HTTP only exposes crypto.getRandomValues", () => {
  vi.stubGlobal("crypto", {
    getRandomValues: globalThis.crypto.getRandomValues.bind(globalThis.crypto),
  });

  const draft = buildAgentOptionsCreateSource({});
  expect(decodeAvatar(draft.initial.avatar)).not.toBeNull();
});
