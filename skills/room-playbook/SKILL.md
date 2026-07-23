---
name: room-playbook
title: Room Collaboration Playbook
description: A general Room Skill example for validating shared Room rule injection.
scope: room
tags: [room, collaboration]
---

# Room Collaboration Playbook

This is a general Room Skill example that shows how a Room can inject shared collaboration rules. Members maintain the concrete workflow from context; the platform only projects the rules into each member runtime.

## Rules

- Collaborate toward the Room goal and do not leak private-context information into the public feed.
- Use Room actions when a member needs to be notified privately.
- For serial workflows, wake only the next member at each step.
