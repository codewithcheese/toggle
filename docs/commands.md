# Commands

Build from the project checkout with `go build -o toggle ./cmd/toggle`. Go 1.27.2 or newer is required to build; the resulting executable requires Git at runtime.

```text
toggle list
toggle status
toggle on <feature> [--dry-run]
toggle off <feature> [--dry-run]
```

`toggle --help` prints usage. Quote names that contain spaces. For names beginning with a dash, use `toggle on -- -name` (put `--dry-run` before `--` for a preview).

## List

Shows all available features with their selected directory paths, whether on or off. Within a Git repo, local definitions override global definitions of the same name. Outside Git, only global definitions appear. Missing discovery roots contribute no features and are not created.

## Status

Shows available features and recorded installations for the containing repo. It works from the root or any subdirectory. Outside Git it fails with a diagnostic.

| Label | Meaning |
|---|---|
| `off` | Not installed; on can pass validation. |
| `on` | Installed files match their at-on content and executable modes; off can pass validation. |
| `modified` | An installed file is missing or changed, but off can pass validation. Existing edits match the currently selected source. |
| `conflicted` | The next transition cannot pass validation: off when installed, on otherwise. Relevant paths and reasons follow the label. |

Conflicts take precedence over modifications. Changes to sources alone do not change an installation's baseline. Status is read-only; displaying a blocked transition does not execute it.

## On and off

Run these commands, including previews, at the Git repo root. They reject subdirectories and locations outside Git. On copies whole source files and preserves executable bits. Off removes recorded installed files after checking edit preservation. Repeated on/off commands are harmless and do not refresh changed definitions.

`--dry-run` displays proposed file, local exclusion, and bookkeeping changes without writing them. It reports the same known conflicts as the real operation. Paths and reasons are provided for comparison with external tools; toggle does not generate diffs.

Command errors return nonzero. Write-failure diagnostics identify partial effects and inspection locations. See [the file lifecycle](lifecycle.md) for edit preservation and failure handling, [feature creation](features.md) for source layouts, and [Git isolation](git-isolation.md) for exclusion and record locations.
