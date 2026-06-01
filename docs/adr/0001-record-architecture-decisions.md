# ADR 0001: Record architecture decisions

## Status

Accepted

## Date

2026-06-01

## Context

DocLens is at the start of its life. We are about to make several decisions that will be hard to reverse later: monorepo vs polyrepo, language splits, storage strategy, AI provider strategy, and so on.

Decisions made without context tend to be questioned every time a new contributor joins. Without a paper trail, the original reasoning is lost and the decision drifts.

We want to record the _why_ alongside the _what_, in a format that is cheap to write and easy to read.

## Decision

We use Architectural Decision Records in the form described by Michael Nygard. ADRs are short Markdown files in `docs/adr/`, numbered, immutable once accepted, and linked from `PROJECT.md`.

Every architecturally significant decision gets an ADR. Decisions local to a single file or that do not affect cross-cutting concerns do not.

## Alternatives Considered

- **No ADRs, rely on commit messages.** Lossy. Hard to find later. Commit messages describe what changed, not why an option was chosen over alternatives.
- **A wiki.** Tends to drift from the code. ADRs in the repo travel with the code and are reviewable in PRs.
- **A single design document.** Becomes stale quickly and merges poorly.

## Consequences

**Positive**

- Decisions are discoverable and reviewable.
- New contributors can learn the system by reading ADRs in order.
- Disagreements happen in PR review against a written proposal, not in retro.

**Negative**

- Discipline required: writing an ADR takes 20–60 minutes.
- Risk of ADR sprawl if used for trivial decisions.

**Neutral**

- ADR numbers are forever; we accept gaps when an ADR is superseded.
