# Commands

Build with Go 1.27.2 or newer: `go build -o toggle ./cmd/toggle`. Git is required at runtime.

```text
toggle list
toggle status
toggle on <feature> [--store <name>] [--dry-run]
toggle off <feature> [--store <name>] [--dry-run]
toggle --help
```

`toggle --help` explains features, stores, copying, and Git-local exclusions before command syntax. It includes the config path, a JSON example, source layout, selectors, root requirements, and optional worktree guidance. It works outside Git and with missing or malformed config. Quote names containing spaces. For names starting with a dash, put flags before `--`, for example `toggle on --store team -- -review`.

## Config and selection

Configure store directories in `~/.config/toggle/config.json`:

```json
{"stores":{"team":"/absolute/path/to/feature-repo"}}
```

Each store contains immediate `<feature>/source/` directories. Store paths must be absolute and names must be nonblank; no names are reserved. Missing config or an empty stores map produces an empty catalog. Malformed config is an error. This user config applies to every project, so repo-specific stores also appear in other projects.

Bare names require exactly one available definition. Multiple definitions are ambiguous for both on and off, regardless of what repo files match. Use `--store <name>` to select a named store explicitly. Explicit selection does not require unrelated external stores to be available; bare selection, list, and status require all configured sources to be readable. Toggle reads current filesystem contents, including uncommitted changes. You manage source directories and Git workflows; Toggle does not clone or update stores.

## List

Lists every definition with its feature name, usable selector, and source directory. Duplicate names remain separate rows. Only configured stores are discovered, from the repo root, subdirectories, or outside Git. Discovery changes no files.

## Status

Compares available definitions against existing repo paths. It reports:

- **Ready for off:** every selected file exists, matches the source bytes/executable bits, and passes off's Git/path checks.
- **Blocked:** a definition has at least one existing destination, but partial presence, a differing file, an incompatible type, or a protection failure blocks off. This includes zero exact matches, such as an edited single-file skill.

Rows include source selectors, paths, and matching/missing/different counts. Entirely absent definitions are omitted. Several definitions can compare against the same paths. These are comparisons, not claims about who installed files or whether every active setup was found.

Status works from the repo root or a subdirectory and requires Git. A completed scan returns zero even when comparisons are blocked. Invalid config, unavailable/unreadable sources, and scan errors return nonzero. Status writes nothing and caches nothing between commands.

## On and off

On/off, including `--dry-run`, require the Git repo root. On copies missing files and accepts matches. Off requires a complete match or complete absence; partial presence conflicts before changes. Matching includes executable mode, and tracked-file/path protections always apply.

`--dry-run` displays proposed file, empty-directory, and exact exclusion changes without applying them. Real commands perform fresh validation. Repeating operations with stable matching/absent inputs and correct exclusions makes no changes.

Conflicts and command failures return nonzero with the feature, source, paths, and reason. Toggle prints no content diff. Unexpected I/O failures may leave partial effects and stop later writes; diagnostics identify completed, failed, and remaining effects and the exclusion location.

See [feature creation](features.md) for the optional worktree pattern and [the file lifecycle](lifecycle.md) for matching, Git-local exclusions, and cleanup.
