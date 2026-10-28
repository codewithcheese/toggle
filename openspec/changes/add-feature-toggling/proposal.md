# Proposal

## Why

Different tasks call for different skills, agents, and configuration. Developers need to bring the appropriate setup into a repo when needed, set it aside when finished, and reuse it across projects without making it permanent project setup or repeatedly managing the files by hand.

## What Changes

- Add a Go CLI with `toggle list`, `toggle status`, `toggle on <name>`, and `toggle off <name>`, with on/off previews through `--dry-run`.
- Discover global features in `~/.local/share/toggle/` and repo-local features in `.toggle/`. The directory name identifies the feature; its `source/` tree mirrors repo paths. Top-level files are reserved for metadata and are not installed.
- Let repo-local features override global features of the same name. Users and agents create features by writing ordinary directories and files, without a manifest or authoring command.
- Install and remove whole files per repo, with harmless repeated commands. Before off removes edited content, require it to match the selected source so the user or agent explicitly decides where to preserve it.
- Protect tracked files, reserve `.git` paths and `.gitignore` installation targets, keep installed copies locally ignored, and report conflicts by feature, path, and reason. Keep installation bookkeeping in Git-local metadata.
- Show feature status from anywhere inside the current Git repo, with a clear error outside Git. On/off operations and previews run at the Git repo root.

The [specs](specs/) define the detailed behavior. [JSON/JSONC editing is deferred](../add-json-configuration/proposal.md); this change copies JSON files whole like other files.

**Scope boundary:** No automatic edit cache, source updates, or diff rendering. Capture, arbitrary-depth feature grouping, named status filtering, linked-worktree coordination, source snapshots, and multi-file recovery machinery remain deferred. If no global or repo-local definition remains for an installed feature, cleanup is manual. Owners choose whether to commit or ignore `.toggle/` using normal Git configuration; toggle does not manage that choice.

## Capabilities

### New Capabilities

- `feature-discovery`: Find global and repo-local features by directory name, select local overrides, and resolve source files to repo paths.
- `feature-toggling`: Turn features on and off, protect unpreserved edits, report status and conflicts, and preview changes.
- `git-isolation`: Protect tracked files and keep installed feature files out of ordinary Git staging.

### Modified Capabilities

None. This is a new project with no existing capability specs.

## Impact

- The Go module gains the CLI, filesystem discovery and copying, Git queries, installation bookkeeping, tests, and documentation.
- Target repos gain or lose installed files and exact entries in Git's local exclude file. One Git-local bookkeeping file records installations.
- Global sources live outside repos; optional repo-local sources live in `.toggle/` and remain under the owner's control.
- Git is a runtime dependency. This scope needs no manifest parser, diff library, or JSON/JSONC editing dependency; internal bookkeeping can use Go's standard JSON support.
