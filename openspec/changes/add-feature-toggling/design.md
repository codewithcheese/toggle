# Design

## Context

The [proposal](proposal.md) explains the problem and scope. The three [capability specs](specs/) define behavior. This is a new Go module with no implementation or existing installation records to migrate.

Features can be shared globally or authored in `.toggle/`. Installed copies can differ from their sources. Git's tracking and ignore rules, filesystem aliases, and the need to protect edits shape the approach below. Specialized [JSON/JSONC editing is deferred](../add-json-configuration/proposal.md); internal JSON bookkeeping is separate from that feature.

## Goals / Non-Goals

Use a common read-only preparation path for application, previews, and status. Keep enough installation information to distinguish edits and safely remove installed files without storing file snapshots.

Operations are synchronous. The design provides inspectable partial failures, without a cross-file transaction, automatic rollback, or recovery journal.

## Decisions

### Use Go and delegate Git semantics to Git

Go's standard library supplies file copying, hashing, JSON serialization, and subprocess execution. Rust was considered; its extra language complexity brings little benefit for this scope. No manifest or diff parser is needed.

Invoke Git with argument lists to locate the working tree and metadata and query tracking and ignore rules. This avoids reproducing Git behavior in a library. Git is required at runtime. Resolve the local exclude path and `toggle.json` through Git's metadata-path facilities, rather than constructing paths assuming `.git` is a directory.

Separate repo lookup from command-specific location checks. Discovery and status use the containing repo's root even when invoked from a subdirectory. Status requires a repo; list can use global definitions outside Git. Apply the working-directory-equals-root guard only to on/off and their previews, so status can reuse transition preparation without inheriting that guard.

### Derive features and destinations from directories

Read immediate feature directories under `~/.local/share/toggle/` and `<repo>/.toggle/`; a directory containing `source/` is a feature and its basename is its identity. Build the available-feature map by loading global definitions, then replacing same-named entries with local definitions. Keep the selected directory in listing output and diagnostics.

Walk only the selected `source/` tree and carry each file's relative path unchanged into the repo. An explicit manifest would duplicate the directory name and tree structure. Top-level metadata has no required format and is excluded from installation by construction. Filesystem traversal, rather than YAML parsing or glob expansion, is sufficient.

Copy file contents and executable modes rather than linking installed files to shared sources. This keeps repo edits independent of the global feature. Users preserve edits by explicitly updating the selected global source or authoring a local feature with the same layout. Toggle never copies edits into a hidden cache or changes definitions itself.

### Resolve paths once and verify them again before effects

For each source file, retain the relative source path, declared repo destination, and resolved destination. Resolve existing symlink components even where the final file is absent. Source resolution must remain inside `source/`; destination validation excludes paths outside the repo, Git metadata, and `.toggle/`, including aliases of these locations. Check declared and resolved repo-relative path components for `.git` at any depth, and reject filenames `.gitignore` at any depth.

Reserving these Git control paths keeps existing ignore rules stable throughout installation. Allowing incoming `.gitignore` files would require evaluating future ignore rules as well as current ones. This version rejects those targets so preparation can remain read-only and Git can evaluate the repo's existing rules directly.

Use resolved destinations as identity for overlap checks and exclusions. Git tracking checks cover both declared and resolved paths. Destination symlink entries are preserved; writes and removals address the resolved files. This permits existing in-repo aliases without allowing a later retargeted symlink to redirect cleanup.

Off starts with recorded installed destinations, not a new source walk to choose deletion targets. Re-resolve their declared paths and compare with the recorded destinations. The current selected source is used only to check explicit preservation of edits at the same relative paths. Unchanged installed files can be removed even if that source has changed or no longer contains the file.

### Prepare changes separately from applying them

```text
selected source + Git queries + installation bookkeeping
  -> source/destination identities and current file comparisons
  -> planned file, exclusion, and bookkeeping changes
  -> complete validation and path-based diagnostics
  -> preview, status classification, or application
```

Preparation creates no files or directories. It collects all known blockers before application, including file-type/content conflicts, destination ownership, boundaries, tracking, and effective ignoring. Validate proposed exact exclusion rules against the repo's existing ignore rules before persisting them.

Compare installed content and executable mode against the at-on baseline first. If changed, off can proceed only if the current selected source matches both. Missing installed files need no removal. Normal off requires an available definition: removing a local override selects the remaining global definition, while losing both blocks normal off and requires manual cleanup. These checks protect edits without saving their contents.

Status prepares off for installed features and on otherwise, then uses the precedence defined in the [toggling spec](specs/feature-toggling/spec.md#requirement-classify-status-with-conflicts-first). Already-on commands remain no-ops; status checking a possible off does not make repeated on an update operation. Formatters consume results and never initiate writes or generate diffs.

### Keep one installation file in Git metadata

Use one JSON file at the Git-resolved `toggle.json` location, commonly `.git/toggle.json`, with entries keyed by feature name. The presence of a feature entry identifies an installation. Successful off removes that entry while preserving other features' entries. No external repo identity or hashed state directory is needed.

| Recorded information | Purpose |
|---|---|
| Feature name and source directory selected at on | Identify the installation and show its origin without pinning future source selection |
| Source-relative file path, content hash, executable mode at on | Detect edits against a fixed baseline without snapshots |
| Declared and resolved destinations | Select removals, detect overlap, and reject redirection |
| Feature exclusion-block identity and exact entries | Remove only that installation's rules |

The at-on source path is diagnostic context. Off uses the currently selected definition, including a new local override, when checking preservation. A source change alone does not change the recorded installed baseline. Whole file contents are never stored in this record.

Git-local storage follows the checkout's metadata without a separately maintained lookup key and keeps bookkeeping out of project files. External per-repo storage would require another identity and lookup scheme. Use ordinary JSON replacement for the small shared record, preserving other feature entries; no database is needed. Linked-worktree coordination remains outside this change.

### Order writes so failures leave useful evidence

After all read-only validation succeeds, apply **on** in this order:

1. Add the feature's exact local exclusion block.
2. Save installation bookkeeping, including all planned destinations and at-on comparisons.
3. Write source files to resolved destinations, skipping redundant copies for accepted matching files.

This order establishes ignoring before file creation and identifies the intended files before target writes. It can leave a recorded partial installation after an I/O failure. Missing installed files can then appear as modified; the record is evidence for inspection, not an automatic retry mechanism.

Apply **off** in this order:

1. Remove recorded installed files after validating unchanged content or explicit preservation. Skip already absent files.
2. Remove the feature's exclusion block.
3. Remove its installation entry from bookkeeping.

Keeping exclusions until removal avoids exposing installed copies to staging. Keeping the entry until completion leaves the installation identifiable after a partial failure. No edited copies are saved during off.

Use atomic replacement for individual metadata files where practical, without promising atomicity across the operation. Keep the prepared effects and their completion results in memory to render the [partial-application report](specs/feature-toggling/spec.md#requirement-report-partial-application-after-write-failures). The failed write can itself have partial effects; report it as failed without assuming its path is untouched. This is in-memory failure reporting, not a persistent recovery journal. Known validation conflicts have no persistent effects; unexpected I/O failures can have partial effects.

## Risks / Trade-offs

- Copies consume more space than links → keep only installed copies and small hashes/modes, with no edit cache or snapshots.
- Source or destination files can change between inspection and writes → recheck path resolution and file comparisons before effects; stop and report if they no longer match the prepared operation. This is not a concurrent-writer coordination system.
- An installed feature with no available global or repo-local definition cannot be cleaned up normally → report the missing definition and installed paths; cleanup remains manual.
- Partial I/O failure can leave records, exclusions, and files out of step → preserve successful effects and report their paths accurately; do not claim rollback or add recovery machinery.
