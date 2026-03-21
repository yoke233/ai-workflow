---
name: gstack-office-hours
description: A normalized ai-workflow adaptation of gstack's office-hours workflow. Use when a request starts as a vague idea and needs reframing into a concrete product direction, narrow wedge, and execution-ready design doc.
---

# Gstack Office Hours

Use this skill before planning or implementation when the user has an idea, a rough feature, or an unclear product direction.

This is a normalized adaptation of `garrytan/gstack`'s `office-hours` workflow.
It keeps the product-thinking structure, but removes gstack-specific telemetry, local state, and upgrade logic.

## Primary Goal

Turn a vague request into a design artifact that downstream planning and execution can use.

## Core Behavior

1. Push back on the user's initial framing when the underlying problem is more important than the requested feature.
2. Identify the real user pain, not just the surface request.
3. Extract the narrowest viable wedge that can ship and teach us something.
4. Produce a design note that downstream skills can reuse.

## Six Questions To Drive The Session

1. Who is the specific user or operator?
2. What painful thing happens today without this feature?
3. What do they currently do instead?
4. What is the smallest wedge that creates obvious value?
5. What did we observe that is surprising or non-obvious?
6. If this works, what broader product does it imply?

## Output Contract

At the end, write a design note to:

```text
.ai-workflow/artifacts/gstack/office-hours/<date>-<topic-slug>.md
```

The note should include:

1. Problem statement
2. User pain
3. Current workaround
4. Product reframe
5. Narrow wedge recommendation
6. Alternative approaches
7. Key assumptions
8. Open questions

## Artifact Metadata Contract

Default placement: `ThreadMessage.Metadata`.
Do not default this skill to `WorkItem` execution.
Reuse these keys:

1. `artifact_namespace = gstack`
2. `artifact_type = design_doc`
3. `artifact_format = markdown`
4. `artifact_relpath = .ai-workflow/artifacts/gstack/office-hours/<date>-<topic-slug>.md`
5. `artifact_title =` a short human-readable title
6. `producer_skill = gstack-office-hours`
7. `producer_kind = skill`
8. `summary =` a 1 to 2 sentence design-note summary

## Handoff Rule

If the design note is complete, recommend the next appropriate step:

- Use `gstack-plan-ceo-review` if scope and product direction need pressure-testing.
- Use `gstack-plan-eng-review` if the product direction is clear and implementation planning should start.

## Quality Bar

Good output does not repeat the user's wording mechanically.
Good output reframes the problem, sharpens the wedge, and leaves behind a reusable artifact.
