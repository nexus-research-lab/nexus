/**
 * INPUT: Exact Pairing mutation intent and a later owner-scoped Pairing list.
 * OUTPUT: Whether the requested end state is now observable without replaying a write.
 * POS: Pairing read-reconciliation model; unchanged state never proves rejection.
 */
import type {
  CreatePairingPayload,
  PairingView,
  UpdatePairingPayload,
} from "@/lib/api/capability/channel-api";

export type PairingMutationIntent =
  | { kind: "create"; payload: CreatePairingPayload }
  | { kind: "delete"; pairingId: string }
  | {
      kind: "update";
      pairingId: string;
      patch: UpdatePairingPayload;
    };

export type PairingReconcileOutcome = "applied" | "unproven";

export function reconcilePairingIntent(
  intent: PairingMutationIntent,
  items: PairingView[],
): PairingReconcileOutcome {
  if (intent.kind === "delete") {
    return items.some((item) => item.pairing_id === intent.pairingId)
      ? "unproven"
      : "applied";
  }
  if (intent.kind === "update") {
    const current = items.find((item) => item.pairing_id === intent.pairingId);
    return current && pairingPatchReached(current, intent.patch)
      ? "applied"
      : "unproven";
  }
  const current = items.find((item) => pairingMatchesCreateTarget(
    item,
    intent.payload,
  ));
  return current && pairingCreateStateReached(current, intent.payload)
    ? "applied"
    : "unproven";
}

function pairingPatchReached(
  item: PairingView,
  patch: UpdatePairingPayload,
): boolean {
  return (patch.agent_id === undefined || item.agent_id === patch.agent_id)
    && (patch.status === undefined || item.status === patch.status)
    && (
      patch.external_name === undefined
      || (item.external_name ?? "") === patch.external_name
    );
}

function pairingCreateStateReached(
  item: PairingView,
  payload: CreatePairingPayload,
): boolean {
  return item.agent_id === payload.agent_id
    && item.status === (payload.status ?? "active")
    && (item.external_name ?? "") === (payload.external_name ?? "");
}

function pairingMatchesCreateTarget(
  item: PairingView,
  payload: CreatePairingPayload,
): boolean {
  return item.channel_type === payload.channel_type
    && (item.account_id ?? "") === (payload.account_id ?? "")
    && item.chat_type === payload.chat_type
    && item.external_ref === payload.external_ref
    && (item.thread_id ?? "") === (payload.thread_id ?? "");
}
