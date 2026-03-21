---
name: gstack-plan-eng-review
description: A normalized ai-workflow adaptation of gstack's engineering review workflow. Use when a feature or product plan should be turned into a buildable engineering plan with architecture, failure modes, dependencies, and test expectations.
---

# Gstack Plan Eng Review

Use this skill when product direction is sufficiently clear and the next job is to make the work buildable.

This is a normalized adaptation of `garrytan/gstack`'s `plan-eng-review` workflow.

## Primary Goal

Convert a product or feature plan into an engineering-ready plan that can feed DAG generation, work item creation, or thread task decomposition.

## What To Review

1. Architecture boundaries
2. Data flow
3. State transitions
4. Dependency order
5. Failure modes
6. Edge cases
7. Trust boundaries
8. Test coverage expectations

## Expected Review Moves

1. Identify missing architectural assumptions.
2. Force hidden sequencing and dependency decisions into the open.
3. Call out where async, retries, or idempotency matter.
4. Distinguish "must persist" from "can recompute".
5. Make test expectations explicit.

## Output Contract

Write an engineering review artifact to:

```text
.ai-workflow/artifacts/gstack/eng-review/<date>-<topic-slug>.md
```

The artifact should include:

1. System boundary summary
2. Main execution flow
3. Failure and retry matrix
4. State or lifecycle notes
5. Dependencies and sequencing
6. Test strategy
7. Open engineering questions
8. Ready-for-planning verdict

## Artifact Metadata Contract

Default placement: `ThreadMessage.Metadata`.
Only place it on the task side when it is explicitly produced inside thread tasks.
Reuse these keys:

1. `artifact_namespace = gstack`
2. `artifact_type = eng_review`
3. `artifact_format = markdown`
4. `artifact_relpath = .ai-workflow/artifacts/gstack/eng-review/<date>-<topic-slug>.md`
5. `artifact_title =` a short human-readable title
6. `producer_skill = gstack-plan-eng-review`
7. `producer_kind = skill`
8. `summary =` a 1 to 2 sentence engineering review summary

## Integration Hint

If the plan is now stable enough to decompose into steps, this output should be suitable input for planning skills such as `plan-actions` and for the planning service.

## Quality Bar

Do not stop at generic advice like "add tests" or "handle errors".
The review should name the actual failure classes, sequencing concerns, and validation strategy.
