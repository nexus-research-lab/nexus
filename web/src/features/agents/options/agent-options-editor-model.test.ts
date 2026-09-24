import { expect, it } from "vitest";
import { decodeAvatar } from "@/shared/lib/humation/avatar";
import { buildAgentOptionsCreateSource } from "./agent-options-editor-model";

it("creates Agent drafts with a saved Humation identity", () => {
  const draft = buildAgentOptionsCreateSource({});
  expect(decodeAvatar(draft.initial.avatar)).not.toBeNull();
});
