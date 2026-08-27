---
name: room-playbook
title: Room Collaboration Playbook
description: Lightweight default rules for coordinating a Room conversation without confusing chat wakeups with durable WorkGraph responsibility.
scope: room
tags: [room, collaboration]
---

# Room Collaboration Playbook

Use this playbook for lightweight Room collaboration. The member handling the
user request is the coordinator and final publisher; other members contribute or
review when invited. A task-specific Room Skill may replace these roles with a
more precise workflow.

## Rules

- The coordinator states the objective, integrates contributions, resolves
  disagreements, and publishes the final answer. Contributors return evidence or
  a concrete result; they do not assume they own final publication.
- Do not leak private-context information into the public feed. For private
  collection, use `send_message` with `destination=current_room`,
  `visibility=private`, explicit recipients,
  `wake_policy=immediate`, and `reply_route=private`. Set
  `next_reply_route=public` only when the coordinator should naturally publish
  the synthesized result.
- Treat public `@member` as conversation wake only. When durable ownership,
  delivery, dependency, recovery, or acceptance is required, materialize a
  WorkGraph and use `assign_work`; finish with `submit_work` and `review_work`.
- For serial conversation workflows, wake only the next member. For parallel
  accountable work, create separate Work Items and do not assume completion
  order.
- The coordinator stops once the objective is satisfied and no member has a new
  action. A member woken without a useful contribution emits
  `<nexus_room_no_reply/>` instead of a filler message.
- If a member fails, times out, or is unavailable, the coordinator either
  continues with available evidence, reassigns managed work through WorkGraph
  controls, or reports the concrete blocker. Do not wait indefinitely or use a
  raw `@` to disguise failed durable responsibility.
