# Toggle

Toggle lets you bring skills, agent instructions, configuration, and
research files into a project for a task, then remove them when you're done.

Keep the originals in a directory outside your project, called a store.
This keeps unused files out of agents' project searches. Within the store,
group files you use together as a named feature: review might contain a
review skill and its instructions. Turn review on to copy those files
into your project, then off to remove the copies. The originals stay
in the store for reuse.

Toggle keeps the project copies out of normal Git staging through Git's
local exclude file. Off removes their Toggle exclusions too; your
`.gitignore` stays unchanged.

## Usage

```text
toggle list                     List features with stores and source paths.
toggle status                   Show features ready for removal and conflicts.
toggle on <feature> [--store <name>] [--dry-run]
toggle off <feature> [--store <name>] [--dry-run]
toggle --help
```

Run on/off, including previews, from your project's Git repo root.
Status works anywhere within that repo. List also works outside Git.

## Set up a store

Choose or create a directory outside your project. If you keep its files
in Git, two useful setups are:

- Use a separate repository for features you want to reuse across projects.
- For project research and experiments, use an external worktree of an
  orphan branch in that project's repository. The branch has independent
  history; the worktree keeps its files outside the project directory.

Register the directory as personal by creating or editing
`~/.config/toggle/config.json`:

```json
{"stores":{"personal":"/absolute/path/to/features"}}
```

Paths must be absolute. All configured stores, including research stores,
are visible in every project. Use `--store <name>` when several stores
contain a feature with the same name. With no stores, `toggle list` is empty.

Toggle reads current files on disk; edits do not need to be committed.
You manage repositories, worktrees, commits, and sharing with Git.
Stores inside a project also work, but off leaves their source files there.

## Add a feature

In the personal store, create a review directory with the files to copy
under `source/`, arranged at their intended project paths:

```text
/absolute/path/to/features/       Store: originals
  review/
    source/
      .agents/skills/review/SKILL.md
```

Turning review on copies that skill to:

```text
/absolute/path/to/project/        Project: copy
  .agents/skills/review/SKILL.md
```

The directory name review is the feature name; no manifest is needed.
Only `source/` contents are copied.

## Use a feature

From your project's Git repo root:

```sh
toggle list
toggle on review --store personal --dry-run
toggle on review --store personal
toggle status
toggle off review --store personal
```

`--dry-run` checks and shows proposed changes without writing anything.
Status compares project files with the features in your stores. It shows
matches ready for removal and conflicts, without tracking past on/off
commands. A completed scan exits with code 0 even when it reports conflicts.

## Matching and removal

On fills in missing files and leaves matching files in place. It stops
if an existing file has different contents or executable permissions.
Off removes the project copies only when every file matches the current
source. If some copies are present and others missing, or any file
differs, off stops without changes. If all copies are absent, off clears
their Toggle exclusions.

To keep project edits, copy the edited files back to their `source/` paths
in the store before off. To discard them, restore the project copies from
the store. Toggle does not save edits for you. Store updates can also
block removal, even if you haven't edited the project copies.

On/off reject feature paths that target Git-tracked files, `.gitignore`
files, `.git` entries or their contents, Git metadata, or configured feature
directories. These checks happen before any changes; an unexpected
failure while applying changes can still leave partial results.
