---
name: gstack-plan-ceo-review
description: A normalized ai-workflow adaptation of gstack's CEO review workflow. Use when a design or feature plan exists and should be pressure-tested for ambition, scope, user value, and product framing before implementation.
---

# Gstack Plan CEO Review

Use this skill after a rough design or feature note exists, but before implementation begins.

This is a normalized adaptation of `garrytan/gstack`'s `plan-ceo-review` workflow.

## Primary Goal

Review a plan from a product and founder perspective:

1. Is this the right problem?
2. Is the proposed scope too small, too broad, or misframed?
3. Is there a stronger wedge or a more compelling version hiding inside the request?

## Review Modes

Choose the mode that best matches the request:

1. Expansion
   - Push toward the most ambitious credible version.
2. Selective Expansion
   - Keep current scope, but surface specific upgrades worth opting into.
3. Hold Scope
   - Strengthen the existing plan without expanding it.
4. Reduction
   - Cut to the narrowest wedge that still creates learning and value.

## Review Checklist

1. Clarify the real user outcome.
2. Separate the user's request from the deeper product job.
3. Identify what makes the current plan ordinary or weak.
4. Suggest concrete framing or scope changes.
5. Preserve a path to near-term execution.

## Output Contract

Write a CEO review artifact to:

```text
.ai-workflow/artifacts/gstack/ceo-review/<date>-<topic-slug>.md
```

Include:

1. Current framing
2. Better framing
3. Recommended review mode
4. Scope decisions
5. Expansion opportunities
6. Reduction opportunities
7. Risks in current product thinking
8. Final recommendation

## Artifact Metadata Contract

Default placement: `ThreadMessage.Metadata`.
Do not default this skill to `WorkItem` execution.
Reuse these keys:

1. `artifact_namespace = gstack`
2. `artifact_type = ceo_review`
3. `artifact_format = markdown`
4. `artifact_relpath = .ai-workflow/artifacts/gstack/ceo-review/<date>-<topic-slug>.md`
5. `artifact_title =` a short human-readable title
6. `producer_skill = gstack-plan-ceo-review`
7. `producer_kind = skill`
8. `summary =` a 1 to 2 sentence CEO review summary

## Handoff Rule

If the plan is now product-clear, recommend `gstack-plan-eng-review` next.
If the problem itself is still underdefined, recommend another `gstack-office-hours` style pass.

## Quality Bar

Do not act like a passive note-taker.
The review should change the quality of the plan, not merely summarize it.
