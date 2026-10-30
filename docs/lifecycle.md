# Applying and removing current definitions

Run on/off and previews at the Git repo root. A bare feature name must be unique; otherwise select explicitly:

```sh
toggle on review --store team --dry-run
toggle on review --store team
toggle off review --store team --dry-run
toggle off review --store team
```

Toggle compares the current definition with current repo files. A match includes file bytes and executable bits. There is no installation record, stored baseline, exclusive ownership, or obligation to turn off an original source.

| Repo files | On | Off |
|---|---|---|
| Every selected destination absent | Copy the definition | Already off; remove stale selected managed exclusions, if any |
| Every selected destination matches | Accept the files and ensure exclusions | Remove the files and selected exclusions |
| Matching files mixed with absence | Fill missing files | Conflict before changes |
| Any differing content/mode or incompatible type | Conflict before changes | Conflict before changes |

Git-tracked files, unsafe paths, and protected source definitions conflict even when contents match. Known conflicts stop the whole operation before persistent changes. Previews use the same checks and write nothing.

## Forks and edits

An agent can prepare `review/source/` in a configured research store with the desired current files and executable modes. It can then select that definition directly:

```sh
toggle on review --store project-research
toggle off review --store project-research
```

No off against the original store, ownership transfer, force mode, or automatic save step is involved. A differing repo file must be reconciled explicitly with the selected source first. Toggle reports paths and reasons, not generated diffs, and never changes the source definition.

Store updates are current differences too. A repo file copied from an earlier source version can conflict with the current version even if nobody edited the repo copy. Deleted or renamed source paths and previous symlink destinations are not remembered; an agent uses ordinary files and Git history to reconcile leftovers.

Different definitions can match the same files. Off through either removes those matching paths; the other definition has no independent installed state. A complete matching untracked setup can be removed even when toggle never created it.

## Directory cleanup and failures

After a complete-match off removes files, empty parent directories are pruned deepest first. Cleanup retains unrelated entries, the repo root, directory symlink entries, and their directory targets. Sources and their metadata stay outside installation.

Unexpected I/O failures stop further writes and may leave partial effects. On establishes exclusions before file creation. Off removes files and empty parents before subtracting exclusions. Diagnostics report completed effects, the failed path, remaining effects, and the exclusion-file location. The failed write itself may also have partial effects; no rollback is promised.

A subsequent off on partial presence conflicts until the agent reconciles it. Ordinary on can complete still-missing files when present ones match, but a partial differing write remains a conflict. Repeating either operation with stable matching/absent inputs and correct exclusions makes no changes.
