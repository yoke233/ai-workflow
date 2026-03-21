---
name: gstack-document-release
description: A normalized ai-workflow adaptation of gstack's documentation release workflow. Use when shipped code has likely changed project behavior and the docs, README, architecture notes, or changelog should be updated to match reality.
---

# Gstack Document Release

Use this skill after implementation or before shipping when documentation drift is likely.

This is a normalized adaptation of `garrytan/gstack`'s `document-release` workflow.

## Primary Goal

Bring project documentation back in sync with shipped behavior.

## What To Check

1. README
2. Architecture docs
3. Runbooks
4. Configuration docs
5. API docs
6. Changelog or release notes
7. Contributor guidance

## Review Method

1. Start from the code and behavior changes.
2. Identify which docs are now stale, missing, or misleading.
3. Prefer updating the source-of-truth doc instead of adding scattered notes.
4. Produce a concrete update plan or directly update the docs if that is the assigned task.

## Output Contract

Write a documentation release artifact to:

```text
.ai-workflow/artifacts/gstack/document-release/<date>-<topic-slug>.md
```

Include:

1. Changed behavior summary
2. Docs that must be updated
3. Docs that were checked and remain accurate
4. Suggested edits or patch plan
5. Release-note summary

## Artifact Metadata Contract

Default placement: `ThreadMessage.Metadata`.
Only place it on `Run.ResultMetadata` when it is explicitly executed as a concrete post-implementation task.
Reuse these keys:

1. `artifact_namespace = gstack`
2. `artifact_type = doc_update_plan`
3. `artifact_format = markdown`
4. `artifact_relpath = .ai-workflow/artifacts/gstack/document-release/<date>-<topic-slug>.md`
5. `artifact_title =` a short human-readable title
6. `producer_skill = gstack-document-release`
7. `producer_kind = skill`
8. `summary =` a 1 to 2 sentence documentation update summary

## Handoff Rule

If the docs were not updated in the current pass, leave behind an actionable checklist so the work can be materialized into follow-up tasks.

## Quality Bar

Do not merely say "update docs".
Name which docs drifted, what is wrong, and what should replace it.
