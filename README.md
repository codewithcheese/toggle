# toggle

Make skills, agents, and configuration available in a repo when needed, then set them aside as the work changes.

A Go CLI for turning features on and off per repo. A feature is a named directory containing `source/`, whose files mirror their destination paths:

```text
~/.local/share/toggle/review/
  README.md
  source/
    .agents/skills/review/SKILL.md
```

The feature name is `review`. Only the contents of `source/` are installed; the top level is reserved for metadata. No manifest is required. Repo-local features in `.toggle/<name>/` use the same layout and override global features with the same name. The repo owner chooses whether to commit or ignore `.toggle/`.

Commands: `toggle list`, `toggle status`, `toggle on <name>`, and `toggle off <name>`. Status works from the repo root or a subdirectory and reports an error outside Git. On/off and their `--dry-run` previews run at the Git repo root.

Installed copies are locally ignored. Preserve edits explicitly in the currently selected feature source before turning a feature off; a repo-local override takes precedence over the global source. Toggle reports conflicts rather than saving an edit cache. If no global or repo-local definition remains for an installed feature, cleanup is manual.

`.gitignore` installation targets, `.git` entries, and files beneath `.git` are reserved at any depth, including paths reached through symlinks.

Build with Go 1.27.2 or newer:

```sh
go build -o toggle ./cmd/toggle
./toggle --help
```

Git is required at runtime. Read [command usage](docs/commands.md), [creating features](docs/features.md), [edit preservation and the file lifecycle](docs/lifecycle.md), and [Git isolation](docs/git-isolation.md).

The [proposal](openspec/changes/add-feature-toggling/proposal.md), [design](openspec/changes/add-feature-toggling/design.md), and [implementation tasks](openspec/changes/add-feature-toggling/tasks.md) record the change. [Partial JSON/JSONC editing is deferred](openspec/changes/add-json-configuration/proposal.md); JSON files can be copied whole.
