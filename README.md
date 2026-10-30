Toggle lets you bring skills, agent instructions, configuration, and
research files into a project for a task, then remove them when you're done.

For code reviews, for example, you could keep a review skill and its
instructions together as a feature named review. Keep your features in
a directory called a store. Turn review on to copy its files from the
store into your project, then off to remove the project copies. The
originals stay in the store for the next time you need them.

The project copies are excluded from normal Git staging through Git's
local exclude file. Your .gitignore stays unchanged.

Usage:
  toggle list                     List features with stores and source paths.
  toggle status                   Show features ready for removal and conflicts.
  toggle on <feature> [--store <name>] [--dry-run]
  toggle off <feature> [--store <name>] [--dry-run]
  toggle --help

Run on/off, including previews, from your project's Git repo root.
Status works anywhere within that repo. List also works outside Git.
A completed status scan exits with code 0 even when it reports conflicts.

Set up a store:
  Create or edit ~/.config/toggle/config.json to name your store:

    {"stores":{"personal":"/absolute/path/to/features"}}

  Paths must be absolute. You can use an ordinary directory, a Git checkout,
  or a worktree. Toggle reads the files as they are on disk; edits do not
  need to be committed.

  All configured stores are visible in every project. If a feature name
  appears in several stores, use --store <name> to choose one.
  With no configured stores, toggle list is empty.

  Keeping stores outside your project keeps inactive feature files out
  of project searches. Configured stores inside a project also work.

Add a feature:
  In the personal store above, arrange the review files like this:

    /absolute/path/to/features/
      review/
        source/
          .agents/skills/review/SKILL.md

  The review directory names the feature. Files under source/ are copied
  to the same paths in your project: this skill becomes
  .agents/skills/review/SKILL.md. No manifest is needed, and files outside
  source/ are not copied.

Try it from your project's Git repo root:
  toggle on review --store personal --dry-run
  toggle on review --store personal
  toggle status
  toggle off review --store personal

  --dry-run checks and shows the proposed changes without writing anything.

Matching and protection:
  Existing project files must match the source contents and executable
  permissions. On fills in missing files and leaves matching files in place.
  Off removes the project copies only if all match the current source.
  If all are already absent, off just clears their Toggle exclusions.
  A mix of present and missing files, or any differing file, blocks off.

  If you've edited project files, save any work you want to keep and resolve
  differences with the store before using off. Toggle does not save edits
  back to the store for you. If the store has changed since you copied a
  feature, off compares your project files with that updated source.

  Feature files cannot be installed or removed at Git-tracked paths,
  .gitignore files, .git entries and descendants, Git metadata, or configured
  feature directories.

  Commands check for conflicts before writing. An unexpected failure while
  applying changes can leave partial results.

Project-specific features:
  For research or experiments tied to one project, you can keep a store
  in an external Git worktree of an orphan branch. This gives the feature
  files independent history in the same repository. Configure the worktree
  directory like any other store; manage worktrees and sharing with Git.
