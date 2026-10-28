# Tasks

This checklist implements the agreed whole-file scope. Specialized [JSON/JSONC editing is deferred](../add-json-configuration/proposal.md). The specs define completion; the design explains the implementation approach.

## 1. CLI entry and feature discovery

References: [feature discovery](specs/feature-discovery/spec.md), [status locations](specs/feature-toggling/spec.md#requirement-inspect-status-from-anywhere-inside-the-repo), and [Git repo lookup](design.md#use-go-and-delegate-git-semantics-to-git).

- [x] 1.1 Add the CLI entry point and parsing for list, status, on, off, and on/off dry runs; verify accepted/rejected arguments with focused tests and a successful build.
- [x] 1.2 Implement read-only containing-repo lookup independently of command-specific root guards; test root/subdirectory resolution and absence of a repo outside Git without creating files.
- [x] 1.3 Implement flat global and repo-local discovery and contextual list output using repo lookup; test directory-derived names, no-manifest discovery, local precedence, global fallback after removing a local override, on/off-independent listing, missing roots, and listing outside Git without writes.
- [x] 1.4 Document creating global and local features with ordinary files and contextual listing; verify examples use mirrored source paths, reserve the top level for metadata, require no manifest or authoring command, and find local features from repo subdirectories.

## 2. Paths, Git queries, and bookkeeping

References: [path identity and boundaries](specs/feature-discovery/spec.md), [Git isolation](specs/git-isolation/spec.md), and [installation storage](design.md#keep-one-installation-file-in-git-metadata).

- [x] 2.1 Implement read-only source traversal and declared/resolved destination handling; test mirrored paths, executable modes, source escapes, repo boundaries, Git metadata, reserved .toggle paths, .git entries and .gitignore targets at any depth, and internal symlink aliases.
- [x] 2.2 Implement Git metadata-location, tracking, and ignore queries using the existing repo lookup; test a .git file pointing to a separate Git directory, tracked declared/resolved paths, and overriding ignore rules using disposable repos.
- [x] 2.3 Implement Git-local JSON bookkeeping for installation entries, source associations, hashes/modes, destination paths, and exclusion ownership; test round trips, repo separation, storage in a separate Git directory, preservation of other entries, and reads that create no files.
- [x] 2.4 Implement exact exclusion-block preparation and writing; test effective Git ignoring, escaped literal file paths, user/other-feature rule preservation, and removal of only the feature's block.
- [x] 2.5 Document Git-local records and installed-file exclusions; verify examples distinguish installed copies from owner-managed .toggle definitions and show where to inspect bookkeeping.

## 3. Read-only on/off preparation

References: [toggling behavior](specs/feature-toggling/spec.md) and [preparation design](design.md#prepare-changes-separately-from-applying-them).

- [x] 3.1 Prepare on changes from the selected source and existing installation entries; test matching untracked-file acceptance, differing/type conflicts, overlap within/across features and aliases, tracked-file rejection before exclusions, and already-on no-ops without writes.
- [x] 3.2 Prepare off changes from recorded destinations; test unchanged, missing, source-preserved, and unpreserved edited files, content/mode comparisons, source drift, new local overrides, fallback after removing a local definition, loss of both definitions, redirected paths, and already-off no-ops without writes.
- [x] 3.3 Collect path-based diagnostics for the entire prepared operation; test that one known conflict blocks all effects, preparation creates nothing, and diagnostics identify feature, relevant paths, and reason without rendering diffs.

## 4. File application and failure reporting

References: [explicit edit preservation](specs/feature-toggling/spec.md#requirement-require-explicit-preservation-of-edits), [partial-application reporting](specs/feature-toggling/spec.md#requirement-report-partial-application-after-write-failures), [Git isolation](specs/git-isolation/spec.md), and [write ordering](design.md#order-writes-so-failures-leave-useful-evidence).

- [x] 4.1 Apply on as exclusions, bookkeeping, then files; test whole-file copying including JSON/JSONC, executable modes, matching-file acceptance, symlink preservation, idempotence, and bookkeeping-write failure leaving target content untouched.
- [x] 4.2 Apply off as recorded-file removals, exclusion removal, then bookkeeping removal; test explicitly preserved edits, accepted pre-existing files, absent installed files, unchanged files after source drift, unrelated/new-source-path preservation, and no cache or source writes.
- [x] 4.3 Recheck prepared paths/comparisons before effects and report write failures with feature, completed effects, failed path, remaining effects, and bookkeeping/exclusion locations; inject on/off failures after a successful effect, including a write that partly changes its target, and verify later writes stop, no rollback occurs, and diagnostics match the partial state.
- [x] 4.4 Document the file lifecycle and manual conflict/failure handling; verify examples show explicit global or local edit preservation and distinguish validation conflicts with no effects from I/O failures with partial effects.

## 5. Command wiring, previews, and status

References: [previews](specs/feature-toggling/spec.md#requirement-preview-on-and-off-without-changes), [diagnostics](specs/feature-toggling/spec.md#requirement-report-conflicts-by-feature-and-path), [status locations](specs/feature-toggling/spec.md#requirement-inspect-status-from-anywhere-inside-the-repo), and [status classification](specs/feature-toggling/spec.md#requirement-classify-status-with-conflicts-first).

- [x] 5.1 Connect on/off and dry runs to shared preparation and application; test root guards for real/preview commands, unchanged dry-run state, and matching preview/application conflicts.
- [x] 5.2 Render conflicts and application failures with nonzero exits and feature/path/reason context; verify output contains no generated content diffs and accurately reports partial effects and inspection locations.
- [x] 5.3 Implement status over available features and recorded installations; test identical root/subdirectory results, nonzero failure outside Git, all four labels, conflict precedence, preserved/unpreserved edits, missing files/definitions, source drift, redirected paths, and independent repo state without writes.
- [x] 5.4 Complete command documentation; exercise examples in disposable repos to verify list source selection, on/off no-ops, previews, status locations/reasons, and edit preservation in the currently selected source.

## 6. Integrated verification

References: all three [capability specs](specs/) and the [overall preparation flow](design.md#prepare-changes-separately-from-applying-them).

- [x] 6.1 Add a disposable-repo CLI lifecycle test combining global/local list, matching untracked skills, edits, blocked off, explicit local preservation, previews, status, off, and on again; verify local content is reinstalled, sources remain unchanged by toggle, installed copies stay ignored, and .toggle follows owner-managed Git rules.
- [x] 6.2 Run `go test ./...`, `go vet ./...`, and `go build ./...`; verify all pass and review the commands against the agreed specs.
