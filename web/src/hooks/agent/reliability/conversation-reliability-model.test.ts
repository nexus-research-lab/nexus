import { describe, expect, it } from "vitest";

import {
  INITIAL_CONVERSATION_RELIABILITY_STATE,
  reduceConversationReliabilityState,
} from "./conversation-reliability-model";

describe("conversation reliability state", () => {
  it("clears a previous failure when a later command is sent", () => {
    const scoped = reduceConversationReliabilityState(
      INITIAL_CONVERSATION_RELIABILITY_STATE,
      { type: "scope_changed", session_key: "session-1" },
    );
    const failed = reduceConversationReliabilityState(scoped, {
      type: "failure_reported",
      failure: { code: "usage_limited", session_key: "session-1" },
    });

    const recovered = reduceConversationReliabilityState(failed, {
      type: "recovery_observed",
      evidence: { kind: "submission_started", session_key: "session-1" },
    });

    expect(recovered.failure).toBeNull();
  });

  it("keeps an unknown delivery result until exact recovery evidence arrives", () => {
    const scoped = reduceConversationReliabilityState(
      INITIAL_CONVERSATION_RELIABILITY_STATE,
      { type: "scope_changed", session_key: "session-1" },
    );
    const failed = reduceConversationReliabilityState(scoped, {
      type: "failure_reported",
      failure: {
        client_request_id: "request-1",
        code: "delivery_unknown",
        session_key: "session-1",
      },
    });

    const submitted = reduceConversationReliabilityState(failed, {
      type: "recovery_observed",
      evidence: { kind: "submission_started", session_key: "session-1" },
    });
    expect(submitted.failure?.code).toBe("delivery_unknown");

    const accepted = reduceConversationReliabilityState(submitted, {
      type: "recovery_observed",
      evidence: {
        client_request_id: "request-1",
        kind: "request_accepted",
        session_key: "session-1",
      },
    });
    expect(accepted.failure).toBeNull();
  });
});
