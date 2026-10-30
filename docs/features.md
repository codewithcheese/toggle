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

Register existing checkout directories in `~/.config/toggle/config.json`:

```json
{"stores":{"team":"/absolute/path/to/feature-repo"}}
```

Paths must be absolute. Names are nonempty strings with no reserved values. You clone, edit, commit, pull, and push these repositories yourself; toggle only reads their current files. Discovery is flat, so a nested `group/review/source/` is not found unless the configured root is `group/`.

A repo-local `.toggle/review/source/` uses the same layout and needs no user config. The repo owner chooses whether to commit or ignore `.toggle/` using ordinary Git configuration. Toggle does not manage that choice.

`toggle list` shows every definition with its source path and usable selector. Same-named features are separate definitions. A bare name must have exactly one match; neither local nor named stores have precedence:

```sh
toggle on review --local
toggle on review --store team
```

`--local` and `--store` are mutually exclusive. A configured store named `local` is an ordinary named store, selected with `--store local`. Explicit selection does not require unrelated source checkouts to be available. Bare selection and full catalog scans require their sources to be readable.

The former `~/.local/share/toggle/` location has no automatic meaning. Existing definitions there can be moved into an owned checkout and configured explicitly; toggle never migrates them automatically.

Installation targets cannot be `.gitignore` files, `.git` entries or their descendants, Git metadata, or `.toggle/` contents. Feature-definition directories from local/configured sources that overlap the destination repo are protected too, including unselected definitions and aliases. Source files must resolve within their feature's source tree.
