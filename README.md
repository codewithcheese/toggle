# toggle

Make skills, agents, and configuration available in a repo when needed, then remove them when finished.

A Go CLI that compares current repo files with a selected feature definition. Features live in user-managed checkout directories or the target repo's `.toggle/`:

```text
/path/to/feature-repo/
  review/
    README.md
    source/
      .agents/skills/review/SKILL.md
```

A feature's directory name is its name. Only its `source/` contents are installed, at the same repo-relative paths; top-level metadata stays in the definition. No manifest is needed. Repo-local features use `.toggle/<feature>/source/`. The repo owner chooses whether to commit or ignore those definitions.

Configure existing checkout roots in `~/.config/toggle/config.json`:

```json
{"stores":{"team":"/absolute/path/to/feature-repo"}}
```

Manage source repositories with normal Git tools. Toggle does not clone or update them, and there is no implicit global store. Config is optional for local-only use.

```sh
toggle list
toggle on review --store team --dry-run
toggle on review --store team
toggle status
toggle off review --store team
```

Use `--local` for a repo-local definition. Bare names must be unique; ambiguous names require `--local` or `--store <name>`. Neither source takes precedence, and no store names are reserved.

On creates missing files and accepts matching files. Off requires every selected file to match, or every destination to be absent. Partial presence or differing content/executable mode is a conflict before changes. Tracked files and protected paths remain protected. `.gitignore` files, `.git` entries and descendants, Git metadata, and feature-definition directories cannot be installation targets.

Matching is independent of earlier commands: there are no installation records, stored fingerprints, ownership transfers, or original-source off obligations. An agent can prepare a matching local definition and select it directly. Files get exact Git-local exclusions in one path-based block; successful off removes the matching files, empty parents, and selected exclusions. Unrelated files, user ignore rules, and directory symlinks are preserved.

Status identifies current definitions ready for off and blocked comparisons with existing paths, including edited single-file skills. It does not infer installation history or find every unknown setup. A completed scan succeeds even with blocked rows; config/source/scan errors fail.

Build with Go 1.27.2 or newer; Git is required at runtime:

```sh
go build -o toggle ./cmd/toggle
./toggle --help
```

Read [commands](docs/commands.md), [creating features](docs/features.md), [the file lifecycle](docs/lifecycle.md), [Git isolation](docs/git-isolation.md), and [transition guidance](docs/migration.md). [JSON/JSONC editing remains deferred](openspec/changes/add-json-configuration/proposal.md); JSON files are copied whole.

The [named-store proposal](openspec/changes/add-named-stores/proposal.md), [design](openspec/changes/add-named-stores/design.md), and [tasks](openspec/changes/add-named-stores/tasks.md) record this change.
