// INPUT: Display identity and omitted/explicit/unknown workspace scope.
// OUTPUT: Legacy default works only for omission; explicit missing scope never revives runtime/task identity.
// POS: Pure Thread workspace contract; round facts and navigation remain independent.

import { expect, it } from "vitest";
import { buildConversationThreadModel } from "./conversation-thread-model";

it.each([
  [undefined, "display-agent"],
  [" workspace-author ", "workspace-author"],
  [null, null],
  ["", null],
  ["  ", null],
] as const)("resolves workspace scope %s to %s without rewriting Thread identity", (workspaceAgentId, expected) => {
  const model = buildConversationThreadModel({
    agentId: "display-agent", isLoading: false, layout: "desktop", messages: [], navigation: "back", pendingPermissions: [], presentation: "transcript", roundId: "round", sessionKey: "exact-thread", workspaceAgentId,
  });
  expect(model.workspaceAgentId).toBe(expected);
  expect(model.sessionKey).toBe("exact-thread");
  expect(model.rounds[0].roundId).toBe("round");
  expect(model.leadingAction).toBe("back");
});
