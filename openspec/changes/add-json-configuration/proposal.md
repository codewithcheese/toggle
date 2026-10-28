# Proposal

**Status: Deferred.** This draft records future work, not an implementation-ready change. The first version of [feature toggling](../add-feature-toggling/proposal.md) installs whole files. JSON editing has no implementation tasks, design, or delta specs scheduled here.

## Why

Some agent setup belongs inside a shared configuration document rather than a standalone file. Features may eventually need to contribute local JSON/JSONC settings while preserving unrelated configuration, but merge and reversal semantics should not delay the file-only version.

## What Changes

When resumed, propose specialized JSON/JSONC editing and safe reversal alongside ordinary feature files. The earlier pointer-based approach and the later deep-merge alternative below are exploration notes, not a selected implementation.

Direction established before deferral:

- Read contributed values from source files, rather than embedding them in metadata.
- Support JSONC while preserving unrelated settings and comments.
- Require target configuration to be ignored and untracked; toggle does not automatically add an ignore rule for a configuration document it edits.
- Treat arrays as whole values.
- Preserve edits explicitly in a global or repo-local feature definition. During off, reverse an unchanged value or one preserved in the selected source; otherwise report the conflicting path and key for the agent to resolve.
- Let agents use their own comparison tools rather than generating built-in diffs.

### Options to revisit

**Explicit pointers:** the earlier draft used source/target declarations and selected JSON Pointers. This gives explicit boundaries for value replacement and overlap checks, but requires configuration beyond the source document.

**Deep merging:** source JSON would describe the contribution at its mirrored target path. Objects could merge recursively while scalars and arrays replace values. Literal-null behavior, automatic handling by extension, and creation of missing object branches were proposed but not accepted before this work was parked.

### Unresolved questions

- Choose explicit pointers or deep merging, and determine whether feature metadata is needed.
- Decide how files intended for whole-file copying are distinguished from JSON merge inputs.
- Revisit missing target files and parent objects; the earlier approach required both to exist, while deep merging raised the option of adding object branches.
- Define conflict boundaries, previous-value records, and safe reversal when multiple features or users change one document, including cleanup of any newly created object branches.

## Capabilities

### New Capabilities

- `json-configuration`: Contribute and safely reverse feature values in local JSON/JSONC documents.

### Modified Capabilities

None finalized in this draft. Reassess the shipped file-only specs when this work resumes.

## Impact

- Future implementation would add configuration parsing/editing, change comparisons, and previous-value bookkeeping, with corresponding tests and documentation.
- Configuration documents would be edited in place under the chosen safety rules. Source-path conventions and bookkeeping should integrate with the file-only implementation available at that time.
