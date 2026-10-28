# Creating features

A feature is an immediate directory under `~/.local/share/toggle/` or a repo's `.toggle/`. Its directory name is its name. Put installable files under `source/`, mirroring their destination paths:

```text
~/.local/share/toggle/review/
  README.md
  source/
    .agents/skills/review/SKILL.md
```

This defines `review`, which installs `.agents/skills/review/SKILL.md`. The top level is reserved for optional metadata such as README files; toggle installs only files under `source/`. No manifest, name declaration, mapping, or create command is needed. JSON and JSONC are ordinary whole files.

Installation targets cannot be `.gitignore` files or `.git` entries, including files beneath a `.git` directory, at any depth. This also applies to resolved destinations reached through symlinks. A top-level feature `.gitignore` outside `source/` is metadata and is not installed.

Use `.toggle/review/source/` for a repo-specific definition. It replaces the same-named global feature within that repo; sources are not combined. The repo owner decides whether to commit or ignore `.toggle/` with ordinary Git configuration. Toggle adds no ignore rules for definitions.

`toggle list` shows available features and their selected directory paths, whether on or off. It finds local features from anywhere inside the repo. Outside Git it shows global features. Discovery is flat: `group/review/source/` is not discovered from the global root unless `group` itself contains `source/`.

Removing a local override selects the remaining global feature. If no definition remains for an installed feature, cleanup is manual.
