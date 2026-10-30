# Creating features and configuring stores

A feature is an immediate directory containing `source/`. Its directory name is its name. Source files mirror their destination paths:

```text
/path/to/feature-repo/
  README.md
  review/
    README.md
    source/
      .agents/skills/review/SKILL.md
```

Only files under `review/source/` are installed. Top-level metadata stays in the definition. No manifest or create command is needed. JSON and JSONC files are copied whole.

Register store directories in `~/.config/toggle/config.json`:

```json
{"stores":{"team":"/absolute/path/to/feature-repo"}}
```

Paths must be absolute. Names are nonblank strings with no reserved values. Stores can be ordinary directories, checkouts, or worktrees. Toggle reads current source files, including modified and untracked files; no source repository or commit is required. You manage any Git operations yourself. Missing config or an empty stores map produces an empty catalog; malformed config is an error. Discovery is flat, so a nested `group/review/source/` is not found unless the configured root is `group/`.

## Repo-specific features

You can keep research or experiments in an external Git worktree of an orphan branch. The branch has independent history in the same repository, and its authoring directory stays outside the project so inactive context stays out of ordinary project searches. Create and manage it with ordinary Git tools, then register its absolute directory path like any other store. Toggle does not create worktrees, require particular branch names, or manage commits and sharing.

The user config applies to every project. A repo-specific store is therefore visible in other projects too; choose a descriptive name such as `project-research` and use `--store project-research` to select it. Outside-project storage is a recommendation. Explicitly configured in-project stores remain usable, with their feature definitions protected.

## Selection and protection

`toggle list` shows every definition with its source path and usable selector. Same-named features are separate definitions. A bare name must have exactly one match across configured stores:

```sh
toggle on review --store project-research
toggle on review --store team
```

No store names are reserved. Explicit selection does not require unrelated source checkouts to be available. Bare selection and full catalog scans require their sources to be readable.

Installation targets cannot be `.gitignore` files, `.git` entries or their descendants, or Git metadata. Configured feature-definition directories that overlap the destination repo are protected too, including unselected definitions and aliases. Source files must resolve within their feature's source tree.

An unregistered `.toggle` directory has no special meaning: it is neither discovered as a store nor reserved as a destination. If configured as a store, its feature directories receive the same protection as those in any other configured directory.
