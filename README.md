# toggle

Make skills, agents, and configuration available in a repo when needed, then remove them when finished.

A Go CLI that compares current repo files with a selected feature definition. Features live in named directory stores:

```text
/path/to/feature-repo/
  review/
    README.md
    source/
      .agents/skills/review/SKILL.md
```

A feature's directory name is its name. Only its `source/` contents are installed, at the same repo-relative paths; top-level metadata stays in the definition. No manifest is needed.

Configure store directories in `~/.config/toggle/config.json`:

```json
{"stores":{"team":"/absolute/path/to/feature-repo"}}
```

Toggle reads current source files, including modified and untracked files. Stores can be ordinary directories, checkouts, or worktrees; no source repository or commit is required. Missing config or an empty stores map gives an empty catalog.

For repo-specific research or experiments, you can use an external Git worktree of an orphan branch. This keeps inactive context out of project searches and gives it independent history in the same repository. Register its directory like any other store; manage worktrees, commits, and sharing with ordinary Git. Configured stores are visible across projects, so use descriptive names such as `project-research`. Configured in-project stores are also supported, with their feature definitions protected.

```sh
toggle list
toggle on review --store team --dry-run
toggle on review --store team
toggle status
toggle off review --store team
```

Bare feature names must be unique across configured stores; use `--store <name>` when a name is ambiguous. No store names are reserved.

On creates missing files and accepts matching files. Off requires every selected file to match, or every destination to be absent. Partial presence or differing content/executable mode is a conflict before changes. Tracked files and protected paths remain protected. `.gitignore` files, `.git` entries and descendants, Git metadata, and feature-definition directories cannot be installation targets.

Matching is independent of earlier commands: there are no installation records, stored fingerprints, ownership transfers, or original-source off obligations. An agent can prepare a matching definition in a configured store and select it directly. Files get exact Git-local exclusions in one path-based block; successful off removes the matching files, empty parents, and selected exclusions. Unrelated files, user ignore rules, and directory symlinks are preserved.

Status identifies current definitions ready for off and blocked comparisons with existing paths, including edited single-file skills. It does not infer installation history or find every unknown setup. A completed scan succeeds even with blocked rows; config/source/scan errors fail.

Build with Go 1.27.2 or newer; Git is required at runtime:

```sh
go build -o toggle ./cmd/toggle
./toggle --help
```

Read [commands](docs/commands.md), [creating features](docs/features.md), and [the file lifecycle](docs/lifecycle.md). JSON and JSONC files are copied whole.
