# Commands

Build with Go 1.27.2 or newer: `go build -o toggle ./cmd/toggle`. Git is required at runtime.

```text
toggle list
toggle status
toggle on <feature> [--local | --store <name>] [--dry-run]
toggle off <feature> [--local | --store <name>] [--dry-run]
```

`toggle --help` includes the config path, a JSON example, source layouts, selectors, and root requirements. It works outside Git and with missing or malformed config. Quote names containing spaces. For names starting with a dash, put flags before `--`, for example `toggle on --local -- -review`.

## Config and selection

Configure existing checkouts in `~/.config/toggle/config.json`:

```json
{"stores":{"team":"/absolute/path/to/feature-repo"}}
```

Each checkout contains immediate `<feature>/source/` directories. Local definitions are in `<repo>/.toggle/<feature>/source/`. Missing config is fine for local-only use. Store names have no reserved values; `--store local` refers to a configured store named local, while `--local` refers to `.toggle/`.

Bare names require exactly one available definition. Multiple definitions are ambiguous for both on and off, regardless of what repo files match. Neither local nor named sources have precedence. `--local` and `--store` are mutually exclusive. You manage source checkouts with Git; toggle does not clone or update them. The former global store is not automatically discovered.

## List

Lists every definition with its feature name, usable selector, and source directory. Duplicate names remain separate rows. Inside Git, root-relative local discovery also works from subdirectories. Outside Git, only configured stores are listed. Discovery changes no files.

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

See [feature creation](features.md), [the file lifecycle](lifecycle.md), [Git isolation](git-isolation.md), and [manual transition guidance](migration.md).
